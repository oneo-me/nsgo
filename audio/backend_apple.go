//go:build darwin || ios

package audio

import (
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
	once sync.Once
	err  error
}

func init() { targetBackend = appleBackend{} }

func loadAppleAudio() error {
	appleFramework.once.Do(func() {
		_, appleFramework.err = purego.Dlopen("/System/Library/Frameworks/AVFoundation.framework/AVFoundation", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	})
	return appleFramework.err
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
	sound.endToken = observeAppleEnd(player, func() {
		if ended != nil {
			ended()
		}
	})
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
	result.endToken = observeAppleEnd(player, result.ended)
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
	mu       sync.Mutex
	player   objc.ID
	endToken objc.ID
	closed   bool
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
	player, token := sound.player, sound.endToken
	sound.player, sound.endToken = 0, 0
	sound.mu.Unlock()
	removeAppleEnd(token)
	if player != 0 {
		objc.Send[bool](player, appleSel("stop"))
		player.Send(appleSel("release"))
	}
	return nil
}

type applePlayer struct {
	mu        sync.Mutex
	player    objc.ID
	endToken  objc.ID
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
	playerID, token, done := player.player, player.endToken, player.tickDone
	player.player, player.endToken = 0, 0
	player.mu.Unlock()
	removeAppleEnd(token)
	if done != nil {
		<-done
	}
	if playerID != 0 {
		objc.Send[bool](playerID, appleSel("stop"))
		playerID.Send(appleSel("release"))
	}
	return nil
}

func (player *applePlayer) ended() {
	player.mu.Lock()
	if player.closed {
		player.mu.Unlock()
		return
	}
	player.stopProgressLocked()
	player.mu.Unlock()
	if player.callbacks.end != nil {
		player.callbacks.end()
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

func observeAppleEnd(player objc.ID, callback func()) objc.ID {
	block := objc.NewBlock(func(objc.Block, objc.ID) { callback() })
	center := objc.ID(objc.GetClass("NSNotificationCenter")).Send(appleSel("defaultCenter"))
	token := center.Send(appleSel("addObserverForName:object:queue:usingBlock:"),
		appleNSString("AVAudioPlayerDidFinishPlayingNotification"), player, objc.ID(0), block)
	block.Release()
	return token.Send(appleSel("retain"))
}

func removeAppleEnd(token objc.ID) {
	if token == 0 {
		return
	}
	objc.ID(objc.GetClass("NSNotificationCenter")).Send(appleSel("defaultCenter")).Send(appleSel("removeObserver:"), token)
	token.Send(appleSel("release"))
}

func appleSel(name string) objc.SEL { return objc.RegisterName(name) }

func appleNSString(value string) objc.ID {
	bytes := append([]byte(value), 0)
	return objc.ID(objc.GetClass("NSString")).Send(appleSel("stringWithUTF8String:"), unsafe.Pointer(&bytes[0]))
}
