//go:build darwin && !ios

package microphone

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"structs"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

const (
	bufferSeconds = 0.1
	bufferCount   = 3

	kAudioFormatLinearPCM      = 0x6C70636D // 'lpcm'
	kLinearPCMFormatSignedInt  = 1 << 2
	kLinearPCMFormatPacked     = 1 << 3
	authorizationNotDetermined = 0
	authorizationAuthorized    = 3
)

// streamDescription is AudioStreamBasicDescription.
type streamDescription struct {
	_                structs.HostLayout
	sampleRate       float64
	formatID         uint32
	formatFlags      uint32
	bytesPerPacket   uint32
	framesPerPacket  uint32
	bytesPerFrame    uint32
	channelsPerFrame uint32
	bitsPerChannel   uint32
	reserved         uint32
}

// queueBuffer is the start of AudioQueueBuffer.
type queueBuffer struct {
	_        structs.HostLayout
	capacity uint32
	data     unsafe.Pointer
	size     uint32
}

var system struct {
	once          sync.Once
	err           error
	newInput      func(format *streamDescription, callback, user, runLoop, mode uintptr, flags uint32, queue *uintptr) int32
	allocate      func(queue uintptr, size uint32, buffer **queueBuffer) int32
	enqueue       func(queue uintptr, buffer *queueBuffer, packets uint32, descriptions uintptr) int32
	start         func(queue, startTime uintptr) int32
	stop          func(queue uintptr, immediate bool) int32
	dispose       func(queue uintptr, immediate bool) int32
	callback      uintptr
	mediaType     objc.ID
	captureDevice objc.ID
	nextID        atomic.Uintptr
	recordings    sync.Map // of *recording, by id
}

type recording struct {
	samples func([]int16)
	stopped atomic.Bool
}

func load() error {
	system.once.Do(func() {
		toolbox, err := purego.Dlopen("/System/Library/Frameworks/AudioToolbox.framework/AudioToolbox", purego.RTLD_NOW)
		if err != nil {
			system.err = err
			return
		}
		foundation, err := purego.Dlopen("/System/Library/Frameworks/AVFoundation.framework/AVFoundation", purego.RTLD_NOW)
		if err != nil {
			system.err = err
			return
		}
		purego.RegisterLibFunc(&system.newInput, toolbox, "AudioQueueNewInput")
		purego.RegisterLibFunc(&system.allocate, toolbox, "AudioQueueAllocateBuffer")
		purego.RegisterLibFunc(&system.enqueue, toolbox, "AudioQueueEnqueueBuffer")
		purego.RegisterLibFunc(&system.start, toolbox, "AudioQueueStart")
		purego.RegisterLibFunc(&system.stop, toolbox, "AudioQueueStop")
		purego.RegisterLibFunc(&system.dispose, toolbox, "AudioQueueDispose")
		media, err := purego.Dlsym(foundation, "AVMediaTypeAudio")
		if err != nil {
			system.err = err
			return
		}
		system.mediaType = **(**objc.ID)(unsafe.Pointer(&media))
		system.captureDevice = objc.ID(objc.GetClass("AVCaptureDevice"))
		// One callback serves every recording, as callbacks are never freed.
		system.callback = purego.NewCallback(func(user, queue uintptr, buffer *queueBuffer, _ uintptr, _ uint32, _ uintptr) {
			v, ok := system.recordings.Load(user)
			if !ok {
				return
			}
			r := v.(*recording)
			if r.stopped.Load() {
				return
			}
			if buffer.size > 0 {
				r.samples(append([]int16(nil), unsafe.Slice((*int16)(buffer.data), buffer.size/2)...))
			}
			system.enqueue(queue, buffer, 0, 0)
		})
	})
	return system.err
}

// Supported says this target can record.
func Supported() bool { return load() == nil }

// Allowed says the system lets this process record, without asking.
func Allowed() bool {
	return load() == nil && authorization() == authorizationAuthorized
}

func authorization() int {
	return objc.Send[int](system.captureDevice, objc.RegisterName("authorizationStatusForMediaType:"), system.mediaType)
}

// Authorize asks the user the first time, waiting for the answer, so call
// it off the UI. It answers nil once this process may record, and
// ErrDenied when it may not.
func Authorize() error {
	if err := load(); err != nil {
		return fmt.Errorf("microphone: %w", err)
	}
	switch authorization() {
	case authorizationAuthorized:
		return nil
	case authorizationNotDetermined:
	default:
		return ErrDenied
	}
	if err := checkUsageDescription(); err != nil {
		return err
	}
	answer := make(chan bool, 1)
	block := objc.NewBlock(func(_ objc.Block, granted bool) { answer <- granted })
	defer block.Release()
	objc.Send[objc.ID](system.captureDevice, objc.RegisterName("requestAccessForMediaType:completionHandler:"), system.mediaType, block)
	if !<-answer {
		return ErrDenied
	}
	return nil
}

// checkUsageDescription stands in front of the permission prompt: macOS asks
// on behalf of the responsible process, which an app started from another
// app is not, and aborts this one when that process declares no usage
// description. Where the system cannot name that process, it lets the
// prompt go ahead.
func checkUsageDescription() error {
	responsible, err := purego.Dlsym(purego.RTLD_DEFAULT, "responsibility_get_pid_responsible_for_pid")
	if err != nil {
		return nil
	}
	pidPath, err := purego.Dlsym(purego.RTLD_DEFAULT, "proc_pidpath")
	if err != nil {
		return nil
	}
	pid, _, _ := purego.SyscallN(responsible, uintptr(os.Getpid()))
	path := make([]byte, 4096)
	n, _, _ := purego.SyscallN(pidPath, pid, uintptr(unsafe.Pointer(&path[0])), uintptr(len(path)))
	if int32(n) <= 0 {
		return nil
	}
	if !declaresUsage(string(path[:n])) {
		return fmt.Errorf("%w: %s", ErrNoUsageDescription, path[:n])
	}
	return nil
}

// declaresUsage says the app bundle holding the executable has a usage
// description. Keys are plain ASCII in both XML and binary property lists.
func declaresUsage(executable string) bool {
	end := strings.LastIndex(executable, ".app/Contents/")
	if end < 0 {
		return false
	}
	info, err := os.ReadFile(executable[:end] + ".app/Contents/Info.plist")
	return err == nil && bytes.Contains(info, []byte("NSMicrophoneUsageDescription"))
}

// Capture records 16-bit mono at rate hertz, passing samples to the
// callback, on a thread of the system's, until the Stream stops. It may
// wait for the user to answer the permission prompt, so call it off the UI.
func Capture(rate int, samples func([]int16)) (*Stream, error) {
	if err := Authorize(); err != nil {
		return nil, err
	}
	format := streamDescription{sampleRate: float64(rate), formatID: kAudioFormatLinearPCM,
		formatFlags: kLinearPCMFormatSignedInt | kLinearPCMFormatPacked, bytesPerPacket: 2, framesPerPacket: 1,
		bytesPerFrame: 2, channelsPerFrame: 1, bitsPerChannel: 16}
	r := &recording{samples: samples}
	id := system.nextID.Add(1)
	system.recordings.Store(id, r)
	var queue uintptr
	if status := system.newInput(&format, system.callback, id, 0, 0, 0, &queue); status != 0 {
		system.recordings.Delete(id)
		return nil, fmt.Errorf("microphone: AudioQueueNewInput: %d", status)
	}
	var once sync.Once
	stop := func() {
		once.Do(func() {
			r.stopped.Store(true)
			system.stop(queue, true)
			system.dispose(queue, true)
			system.recordings.Delete(id)
		})
	}
	size := uint32(float64(rate)*bufferSeconds) * 2
	for range bufferCount {
		var buffer *queueBuffer
		if status := system.allocate(queue, size, &buffer); status != 0 {
			stop()
			return nil, fmt.Errorf("microphone: AudioQueueAllocateBuffer: %d", status)
		}
		system.enqueue(queue, buffer, 0, 0)
	}
	if status := system.start(queue, 0); status != 0 {
		stop()
		return nil, fmt.Errorf("microphone: AudioQueueStart: %d", status)
	}
	return &Stream{stop}, nil
}
