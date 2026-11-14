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

type playerCallbacks struct {
	progress func(position, duration time.Duration)
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

// targetBackend is replaced by a target's native implementation in a later
// stage. Keeping the boundary here lets the public API and its lifecycle tests
// remain unchanged as those implementations arrive.
var targetBackend backend = unsupportedBackend{}

type unsupportedBackend struct{}

func (unsupportedBackend) openSound(source, Options, func()) (soundBackend, error) {
	return nil, ErrUnsupported
}

func (unsupportedBackend) openPlayer(source, Options, playerCallbacks) (playerBackend, error) {
	return nil, ErrUnsupported
}

type callbackQueue struct {
	mu    sync.Mutex
	cond  *sync.Cond
	items []func()
}

func newCallbackQueue() *callbackQueue {
	queue := &callbackQueue{}
	queue.cond = sync.NewCond(&queue.mu)
	go queue.run()
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
