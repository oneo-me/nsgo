package audio

import (
	"sync"
	"time"
)

type source struct {
	path string
	data []byte
	mime string
}

type backend interface {
	openSound(source, Options, func()) (soundBackend, error)
	openPlayer(source, Options, playerCallbacks) (playerBackend, error)
}

type soundBackend interface {
	setVolume(float64) error
	stop() error
	close() error
}

// playerCallbacks are how a backend reports what the native player does on its own. A backend calls paused when the native player stops without the app asking, such as on an audio-session interruption, a lost audio focus or an unplugged headset.
type playerCallbacks struct {
	progress func(position, duration time.Duration)
	paused   func()
	end      func()
	err      func(error)
}

type playerBackend interface {
	play() error
	pause() error
	stop() error
	seek(time.Duration) error
	setVolume(float64) error
	setRate(float64) error
	setLoop(bool) error
	// position reports where playback is now, including a seek queued before metadata is ready.
	position() time.Duration
	duration() time.Duration
	playing() bool
	close() error
}

// targetBackend is replaced by a target's native implementation, in this package's files for that target, in a later stage. Those files reach the system through purego or syscall, as microphone's do, which is why audio is not among layering_test.go's portable packages; the public API and its lifecycle tests stay unchanged as they arrive.
var targetBackend backend = unsupportedBackend{}

type unsupportedBackend struct{}

func (unsupportedBackend) openSound(source, Options, func()) (soundBackend, error) {
	return nil, ErrUnsupported
}

func (unsupportedBackend) openPlayer(source, Options, playerCallbacks) (playerBackend, error) {
	return nil, ErrUnsupported
}

// callbackQueue runs app callbacks serially on a goroutine of its own, started by the first callback. A player keeps at most one progress report in it, so a slow callback delays reports rather than growing the queue.
type callbackQueue struct {
	start sync.Once
	mu    sync.Mutex
	cond  *sync.Cond
	items []func()
}

func newCallbackQueue() *callbackQueue {
	queue := &callbackQueue{}
	queue.cond = sync.NewCond(&queue.mu)
	return queue
}

func (queue *callbackQueue) run() {
	for {
		queue.mu.Lock()
		for len(queue.items) == 0 {
			queue.cond.Wait()
		}
		callback := queue.items[0]
		queue.items[0] = nil
		queue.items = queue.items[1:]
		queue.mu.Unlock()
		callback()
	}
}

func (queue *callbackQueue) add(callback func()) {
	if callback == nil {
		return
	}
	queue.start.Do(func() { go queue.run() })
	queue.mu.Lock()
	queue.items = append(queue.items, callback)
	queue.cond.Signal()
	queue.mu.Unlock()
}

var callbacks = newCallbackQueue()

func openPlayer(src source, options Options) (*Player, error) {
	player := &Player{}
	handle, err := targetBackend.openPlayer(src, options, playerCallbacks{
		progress: player.progressed,
		paused:   player.pausedByBackend,
		end:      player.endedByBackend,
		err:      player.failed,
	})
	if err != nil {
		return nil, err
	}
	if handle == nil {
		return nil, ErrUnsupported
	}
	player.backend = handle
	return player, nil
}

func playSound(src source, options Options) (*Sound, error) {
	sound := &Sound{}
	handle, err := targetBackend.openSound(src, options, sound.endedByBackend)
	if err != nil {
		return nil, err
	}
	if handle == nil {
		return nil, ErrUnsupported
	}
	sound.backend = handle
	return sound, nil
}
