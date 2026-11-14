//go:build windows

package windows

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"syscall"
	"unsafe"
)

const (
	instanceMutexExists   = 183 // ERROR_ALREADY_EXISTS
	instancePipeConnected = 535 // ERROR_PIPE_CONNECTED
	instanceInvalidHandle = ^uintptr(0)

	instanceGenericWrite        = 0x40000000
	instanceOpenExisting        = 3
	instancePipeAccessInbound   = 0x00000001
	instancePipeTypeMessage     = 0x00000004
	instancePipeReadmodeMessage = 0x00000002
	instancePipeWait            = 0x00000000
	instanceFirstPipeInstance   = 0x00080000
)

var (
	createMutexW        = kernel32.NewProc("CreateMutexW")
	createNamedPipeW    = kernel32.NewProc("CreateNamedPipeW")
	connectNamedPipe    = kernel32.NewProc("ConnectNamedPipe")
	disconnectNamedPipe = kernel32.NewProc("DisconnectNamedPipe")
	createFileW         = kernel32.NewProc("CreateFileW")
	readFile            = kernel32.NewProc("ReadFile")
	writeFile           = kernel32.NewProc("WriteFile")
	closeHandle         = kernel32.NewProc("CloseHandle")
	sleep               = kernel32.NewProc("Sleep")
	processIDToSession  = kernel32.NewProc("ProcessIdToSessionId")
)

// instance is a named mutex and its message pipe. The mutex prevents a second engine even while the first window is still being created; pending pipe messages wait until Run installs the delivery function.
type instance struct {
	mutex uintptr
	name  *uint16
	done  chan struct{}
	once  sync.Once

	mu      sync.Mutex
	deliver func([]string)
	pending [][]string
}

func claimInstance(id string, args []string) (*instance, bool) {
	mutexName, pipeName := instanceNames(id)
	h, _, err := createMutexW.Call(0, 1, uintptr(unsafe.Pointer(mutexName)))
	if h == 0 {
		log.Printf("nsgo: single-instance mutex: %v", err)
		return nil, false
	}
	if err == syscall.Errno(instanceMutexExists) {
		closeHandle.Call(h)
		if !sendInstance(pipeName, args) {
			log.Printf("nsgo: could not hand launch to the running instance")
		}
		return nil, true
	}
	i := &instance{mutex: h, name: pipeName, done: make(chan struct{})}
	go i.serve()
	return i, false
}

// instanceNames are the mutex and pipe of app id in this logon session. Local\ mutexes are per session already; pipe names are machine-wide, so the pipe's carries the session too, or a second user's launch would reach the first user's app.
func instanceNames(id string) (*uint16, *uint16) {
	digest := sha256.Sum256([]byte(id))
	var session uint32
	processIDToSession.Call(uintptr(syscall.Getpid()), uintptr(unsafe.Pointer(&session)))
	key := fmt.Sprintf("NSGo-%x", digest[:12])
	return wide("Local\\" + key), wide(fmt.Sprintf("\\\\.\\pipe\\%s-%d", key, session))
}

func sendInstance(name *uint16, args []string) bool {
	payload, err := json.Marshal(args)
	if err != nil {
		return false
	}
	for range 20 {
		h, _, _ := createFileW.Call(uintptr(unsafe.Pointer(name)), instanceGenericWrite, 0, 0,
			instanceOpenExisting, 0, 0)
		if h != instanceInvalidHandle {
			var written uint32
			_, _, err = writeFile.Call(h, uintptr(unsafe.Pointer(&payload[0])), uintptr(len(payload)), uintptr(unsafe.Pointer(&written)), 0)
			closeHandle.Call(h)
			return err == nil && written == uint32(len(payload))
		}
		// The first process may still be creating its pipe. Retry briefly; a
		// mutex owner is authoritative even if it never reaches its loop.
		sleep.Call(50)
	}
	return false
}

func (i *instance) serve() {
	for {
		// FILE_FLAG_FIRST_PIPE_INSTANCE: only the mutex's owner may serve this name.
		pipe, _, err := createNamedPipeW.Call(uintptr(unsafe.Pointer(i.name)),
			instancePipeAccessInbound|instanceFirstPipeInstance, instancePipeTypeMessage|instancePipeReadmodeMessage|instancePipeWait,
			1, 64<<10, 64<<10, 0, 0)
		if pipe == instanceInvalidHandle {
			log.Printf("nsgo: single-instance pipe: %v", err)
			return
		}
		connected, _, err := connectNamedPipe.Call(pipe, 0)
		var args []string
		received := false
		if connected != 0 || err == syscall.Errno(instancePipeConnected) {
			var data [64 << 10]byte
			var read uint32
			if ok, _, _ := readFile.Call(pipe, uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)), uintptr(unsafe.Pointer(&read)), 0); ok != 0 {
				received = json.Unmarshal(data[:read], &args) == nil
			}
			disconnectNamedPipe.Call(pipe)
		}
		closeHandle.Call(pipe)
		select {
		case <-i.done:
			return
		default:
		}
		if received {
			i.enqueue(args)
		}
	}
}

func (i *instance) enqueue(args []string) {
	i.mu.Lock()
	if i.deliver == nil {
		i.pending = append(i.pending, args)
		i.mu.Unlock()
		return
	}
	deliver := i.deliver
	i.mu.Unlock()
	deliver(args)
}

func (i *instance) setDelivery(deliver func([]string)) {
	i.mu.Lock()
	i.deliver = deliver
	pending := i.pending
	i.pending = nil
	i.mu.Unlock()
	for _, args := range pending {
		deliver(args)
	}
}

// close gives up the mutex and ends serve: it connects to the pipe once, so the ConnectNamedPipe serve waits in returns and serve closes its own handle.
func (i *instance) close() {
	if i == nil {
		return
	}
	i.once.Do(func() {
		close(i.done)
		if h, _, _ := createFileW.Call(uintptr(unsafe.Pointer(i.name)), instanceGenericWrite, 0, 0,
			instanceOpenExisting, 0, 0); h != instanceInvalidHandle {
			closeHandle.Call(h)
		}
		closeHandle.Call(i.mutex)
	})
}
