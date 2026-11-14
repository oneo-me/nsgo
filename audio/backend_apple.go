//go:build darwin || ios

package audio

import (
	"fmt"
	"sync"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

// The Apple backend uses AVAudioPlayer for both cues and local long-form
// files. It keeps all Objective-C objects behind the audio backend boundary;
// the public wrappers still serialize callbacks on audio's callback queue.
type appleBackend struct{}

var appleFramework struct {
	once     sync.Once
	err      error
	delegate objc.ID
}

// appleDelegates maps each open AVAudioPlayer to what its delegate reports
// to. AVAudioPlayer posts no notifications: it reports the end of playback
// and decoding errors only to its delegate, which one shared object of the
// class registered in loadAppleAudio stands for. A player leaves the map
// before it is released, so a report arriving late reaches nothing.
var appleDelegates struct {
	sync.Mutex
	players map[objc.ID]appleEvents
}

// appleEvents are what an AVAudioPlayer's delegate reports: the end of
// playback, ok when it was not cut short by a decoding error, and a
// decoding error.
type appleEvents struct {
	finished func(ok bool)
	failed   func(error)
}

func init() { targetBackend = appleBackend{} }

func loadAppleAudio() error {
	appleFramework.once.Do(func() {
		if _, err := purego.Dlopen("/System/Library/Frameworks/AVFoundation.framework/AVFoundation", purego.RTLD_NOW|purego.RTLD_GLOBAL); err != nil {
			appleFramework.err = err
			return
		}
		var protocols []*objc.Protocol
		if protocol := objc.GetProtocol("AVAudioPlayerDelegate"); protocol != nil {
			protocols = append(protocols, protocol)
		}
		class, err := objc.RegisterClass("NSGoAudioPlayerDelegate", objc.GetClass("NSObject"), protocols, []objc.FieldDef{}, []objc.MethodDef{
			{Cmd: appleSel("audioPlayerDidFinishPlaying:successfully:"), Fn: func(_ objc.ID, _ objc.SEL, player objc.ID, ok bool) {
				appleReport(player, func(events appleEvents) { events.finished(ok) })
			}},
			{Cmd: appleSel("audioPlayerDecodeErrorDidOccur:error:"), Fn: func(_ objc.ID, _ objc.SEL, player objc.ID, nsError objc.ID) {
				err := fmt.Errorf("%w: decoding failed", ErrUnsupportedFormat)
				if detail := appleErrorText(nsError); detail != "" {
					err = fmt.Errorf("%w: %s", ErrUnsupportedFormat, detail)
				}
				appleReport(player, func(events appleEvents) { events.failed(err) })
			}},
		})
		if err != nil {
			appleFramework.err = err
			return
		}
		// The delegate property is weak; this object is never released.
		appleFramework.delegate = objc.ID(class).Send(appleSel("new"))
		appleDelegates.players = map[objc.ID]appleEvents{}
	})
	return appleFramework.err
}

// appleWatch makes the shared delegate report player's events to events.
func appleWatch(player objc.ID, events appleEvents) {
	appleDelegates.Lock()
	appleDelegates.players[player] = events
	appleDelegates.Unlock()
	player.Send(appleSel("setDelegate:"), appleFramework.delegate)
}

// appleUnwatch stops reporting player's events; reports already on their
// way are dropped.
func appleUnwatch(player objc.ID) {
	appleDelegates.Lock()
	delete(appleDelegates.players, player)
	appleDelegates.Unlock()
}

// appleReport hands a delegate report to player's events off the thread
// AVAudioPlayer reports on, the main thread, so a backend's lock or the
// callback queue never holds it up.
func appleReport(player objc.ID, report func(appleEvents)) {
	appleDelegates.Lock()
	events, ok := appleDelegates.players[player]
	appleDelegates.Unlock()
	if ok {
		go report(events)
	}
}

// appleRelease detaches player from the delegate, stops it and releases it.
func appleRelease(player objc.ID) {
	player.Send(appleSel("setDelegate:"), objc.ID(0))
	objc.Send[bool](player, appleSel("stop"))
	player.Send(appleSel("release"))
}

func (appleBackend) openSound(src source, options Options, ended func()) (soundBackend, error) {
	if err := loadAppleAudio(); err != nil {
		return nil, ErrUnsupported
	}
	// Sounds are always one-shot; WithLoop only applies to Player.
	options.Loop = false
	player, err := newApplePlayer(src, options)
	if err != nil {
		return nil, err
	}
	sound := &appleSound{player: player}
	// A cue ends, as far as its app is concerned, whether it played to the
	// end or a decoding error cut it short; Sound reports the end once.
	end := func() {
		if ended != nil {
			ended()
		}
	}
	appleWatch(player, appleEvents{finished: func(bool) { end() }, failed: func(error) { end() }})
	if !objc.Send[bool](player, appleSel("play")) {
		sound.close()
		return nil, ErrDeviceUnavailable
	}
	return sound, nil
}

func (appleBackend) openPlayer(src source, options Options, callbacks playerCallbacks) (playerBackend, error) {
	if err := loadAppleAudio(); err != nil {
		return nil, ErrUnsupported
	}
	player, err := newApplePlayer(src, options)
	if err != nil {
		return nil, err
	}
	result := &applePlayer{player: player, options: options, callbacks: callbacks}
	appleWatch(player, appleEvents{finished: result.finished, failed: result.failed})
	return result, nil
}

func newApplePlayer(src source, options Options) (objc.ID, error) {
	var player objc.ID
	av := objc.ID(objc.GetClass("AVAudioPlayer")).Send(appleSel("alloc"))
	if av == 0 {
		return 0, ErrUnsupported
	}
	if len(src.data) > 0 {
		data := objc.ID(objc.GetClass("NSData")).Send(appleSel("dataWithBytes:length:"), unsafe.Pointer(&src.data[0]), uint(len(src.data)))
		player = av.Send(appleSel("initWithData:error:"), data, objc.ID(0))
	} else {
		url := objc.ID(objc.GetClass("NSURL")).Send(appleSel("fileURLWithPath:"), appleNSString(src.path))
		player = av.Send(appleSel("initWithContentsOfURL:error:"), url, objc.ID(0))
	}
	if player == 0 {
		av.Send(appleSel("release"))
		if len(src.data) > 0 {
			return 0, ErrUnsupportedFormat
		}
		return 0, ErrUnreadableSource
	}
	objc.Send[bool](player, appleSel("prepareToPlay"))
	objc.Send[float32](player, appleSel("setVolume:"), float32(options.Volume))
	objc.Send[bool](player, appleSel("setEnableRate:"), true)
	objc.Send[float32](player, appleSel("setRate:"), float32(options.Rate))
	loops := 0
	if options.Loop {
		loops = -1
	}
	objc.Send[int](player, appleSel("setNumberOfLoops:"), loops)
	return player, nil
}

type appleSound struct {
	mu     sync.Mutex
	player objc.ID
	closed bool
}

func (sound *appleSound) setVolume(volume float64) error {
	sound.mu.Lock()
	defer sound.mu.Unlock()
	if sound.closed {
		return ErrClosed
	}
	objc.Send[float32](sound.player, appleSel("setVolume:"), float32(volume))
	return nil
}

func (sound *appleSound) stop() error {
	sound.mu.Lock()
	defer sound.mu.Unlock()
	if sound.closed {
		return ErrClosed
	}
	objc.Send[bool](sound.player, appleSel("stop"))
	return nil
}

func (sound *appleSound) close() error {
	sound.mu.Lock()
	if sound.closed {
		sound.mu.Unlock()
		return nil
	}
	sound.closed = true
	player := sound.player
	sound.player = 0
	sound.mu.Unlock()
	if player != 0 {
		appleUnwatch(player)
		appleRelease(player)
	}
	return nil
}

type applePlayer struct {
	mu        sync.Mutex
	player    objc.ID
	options   Options
	callbacks playerCallbacks
	closed    bool
	ticking   bool
	stopTick  chan struct{}
	tickDone  chan struct{}
}

func (player *applePlayer) play() error {
	player.mu.Lock()
	if player.closed {
		player.mu.Unlock()
		return ErrClosed
	}
	ok := objc.Send[bool](player.player, appleSel("play"))
	if ok && !player.ticking {
		player.ticking = true
		player.stopTick, player.tickDone = make(chan struct{}), make(chan struct{})
		stop, done := player.stopTick, player.tickDone
		player.mu.Unlock()
		go player.progressLoop(stop, done)
		return nil
	}
	player.mu.Unlock()
	if !ok {
		return ErrDeviceUnavailable
	}
	return nil
}

func (player *applePlayer) pause() error {
	player.mu.Lock()
	if player.closed {
		player.mu.Unlock()
		return ErrClosed
	}
	objc.Send[bool](player.player, appleSel("pause"))
	player.stopProgressLocked()
	player.mu.Unlock()
	return nil
}

func (player *applePlayer) stop() error {
	player.mu.Lock()
	if player.closed {
		player.mu.Unlock()
		return ErrClosed
	}
	objc.Send[bool](player.player, appleSel("stop"))
	objc.Send[float64](player.player, appleSel("setCurrentTime:"), float64(0))
	player.stopProgressLocked()
	player.mu.Unlock()
	return nil
}

func (player *applePlayer) seek(position time.Duration) error {
	player.mu.Lock()
	defer player.mu.Unlock()
	if player.closed {
		return ErrClosed
	}
	objc.Send[bool](player.player, appleSel("setCurrentTime:"), position.Seconds())
	return nil
}

func (player *applePlayer) setVolume(volume float64) error {
	player.mu.Lock()
	defer player.mu.Unlock()
	if player.closed {
		return ErrClosed
	}
	objc.Send[float32](player.player, appleSel("setVolume:"), float32(volume))
	return nil
}

func (player *applePlayer) setRate(rate float64) error {
	player.mu.Lock()
	defer player.mu.Unlock()
	if player.closed {
		return ErrClosed
	}
	objc.Send[bool](player.player, appleSel("setEnableRate:"), true)
	objc.Send[float32](player.player, appleSel("setRate:"), float32(rate))
	return nil
}

func (player *applePlayer) setLoop(loop bool) error {
	player.mu.Lock()
	defer player.mu.Unlock()
	if player.closed {
		return ErrClosed
	}
	loops := 0
	if loop {
		loops = -1
	}
	objc.Send[int](player.player, appleSel("setNumberOfLoops:"), loops)
	return nil
}

func (player *applePlayer) position() time.Duration {
	player.mu.Lock()
	defer player.mu.Unlock()
	if player.closed {
		return 0
	}
	return time.Duration(objc.Send[float64](player.player, appleSel("currentTime")) * float64(time.Second))
}

func (player *applePlayer) duration() time.Duration {
	player.mu.Lock()
	defer player.mu.Unlock()
	if player.closed {
		return 0
	}
	return time.Duration(objc.Send[float64](player.player, appleSel("duration")) * float64(time.Second))
}

func (player *applePlayer) playing() bool {
	player.mu.Lock()
	defer player.mu.Unlock()
	return !player.closed && objc.Send[bool](player.player, appleSel("isPlaying"))
}

func (player *applePlayer) close() error {
	player.mu.Lock()
	if player.closed {
		player.mu.Unlock()
		return nil
	}
	player.closed = true
	player.stopProgressLocked()
	playerID, done := player.player, player.tickDone
	player.player = 0
	player.mu.Unlock()
	if playerID != 0 {
		appleUnwatch(playerID)
	}
	if done != nil {
		<-done
	}
	if playerID != 0 {
		appleRelease(playerID)
	}
	return nil
}

// finished reports the end of playback: the final position, then the end,
// or an error when a decoding error cut playback short.
func (player *applePlayer) finished(ok bool) {
	player.mu.Lock()
	if player.closed {
		player.mu.Unlock()
		return
	}
	player.stopProgressLocked()
	if ok && player.callbacks.progress != nil {
		duration := time.Duration(objc.Send[float64](player.player, appleSel("duration")) * float64(time.Second))
		player.callbacks.progress(duration, duration)
	}
	player.mu.Unlock()
	if !ok {
		player.failed(fmt.Errorf("%w: decoding stopped playback", ErrUnsupportedFormat))
		return
	}
	if player.callbacks.end != nil {
		player.callbacks.end()
	}
}

// failed reports a decoding error, which stops playback.
func (player *applePlayer) failed(err error) {
	player.mu.Lock()
	if player.closed {
		player.mu.Unlock()
		return
	}
	player.stopProgressLocked()
	player.mu.Unlock()
	if player.callbacks.err != nil {
		player.callbacks.err(err)
	}
}

func (player *applePlayer) progressLoop(stop <-chan struct{}, done chan<- struct{}) {
	ticker := time.NewTicker(player.options.ProgressInterval)
	defer ticker.Stop()
	defer close(done)
	for {
		select {
		case <-ticker.C:
			player.mu.Lock()
			select {
			case <-stop:
				// Playback ended or paused while this tick waited for the lock.
				player.mu.Unlock()
				return
			default:
			}
			if player.closed {
				player.mu.Unlock()
				return
			}
			position := time.Duration(objc.Send[float64](player.player, appleSel("currentTime")) * float64(time.Second))
			duration := time.Duration(objc.Send[float64](player.player, appleSel("duration")) * float64(time.Second))
			callback := player.callbacks.progress
			player.mu.Unlock()
			if callback != nil {
				callback(position, duration)
			}
		case <-stop:
			return
		}
	}
}

func (player *applePlayer) stopProgressLocked() {
	if !player.ticking {
		return
	}
	close(player.stopTick)
	player.ticking = false
}

func appleSel(name string) objc.SEL { return objc.RegisterName(name) }

// appleErrorText returns an NSError's localized description, or "" for nil.
func appleErrorText(nsError objc.ID) string {
	if nsError == 0 {
		return ""
	}
	text := objc.Send[*byte](nsError.Send(appleSel("localizedDescription")), appleSel("UTF8String"))
	if text == nil {
		return ""
	}
	n := 0
	for *(*byte)(unsafe.Add(unsafe.Pointer(text), n)) != 0 {
		n++
	}
	return string(unsafe.Slice(text, n))
}

func appleNSString(value string) objc.ID {
	bytes := append([]byte(value), 0)
	return objc.ID(objc.GetClass("NSString")).Send(appleSel("stringWithUTF8String:"), unsafe.Pointer(&bytes[0]))
}
