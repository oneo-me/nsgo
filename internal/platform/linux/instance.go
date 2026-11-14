//go:build linux && !android

package linux

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/nsgo-dev/nsgo/internal/engine"
)

// instance is the Unix socket owned by the first copy of an app. Messages
// received before the GTK window exists wait in pending until Run installs the
// delivery function.
type instance struct {
	listener *net.UnixListener
	path     string
	done     chan struct{}
	once     sync.Once

	mu      sync.Mutex
	deliver func([]string)
	pending [][]string
}

// claimInstance makes this process the owner, or hands its launch to the
// owner and reports that the caller should exit. A failed handoff keeps the
// caller alive only when the old socket is stale; an owner that is listening
// always wins, so two engines are never started for one app ID.
func claimInstance(id string, args []string) (*instance, bool) {
	path, err := instancePath(id)
	if err != nil {
		log.Printf("nsgo: single-instance setup: %v", err)
		return nil, false
	}
	if sendInstance(path, args) {
		return nil, true
	}
	// A dead process can leave the socket pathname behind. Remove it only
	// after all connection attempts failed, then claim it atomically with
	// ListenUnix.
	_ = os.Remove(path)
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		// Another process may have won the race between the failed handoff and
		// our listen. Give that owner one last chance before running a copy.
		if sendInstance(path, args) {
			return nil, true
		}
		log.Printf("nsgo: single-instance listener: %v", err)
		return nil, false
	}
	_ = os.Chmod(path, 0o600)
	i := &instance{listener: listener, path: path, done: make(chan struct{})}
	go i.serve()
	return i, false
}

func instancePath(id string) (string, error) {
	data, _ := engine.AppDirs(id)
	if data == "" {
		return "", errors.New("no data directory")
	}
	if err := os.MkdirAll(data, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(data, "instance.sock")
	// Linux limits Unix socket addresses to about 108 bytes. Keep the stable
	// app directory where it fits, and use a per-user temporary pathname for
	// unusually long XDG paths.
	if len(path) < 100 {
		return path, nil
	}
	digest := sha256.Sum256([]byte(id + "\x00" + data))
	return filepath.Join(os.TempDir(), fmt.Sprintf("nsgo-%d-%x.sock", os.Getuid(), digest[:8])), nil
}

func sendInstance(path string, args []string) bool {
	for range 8 {
		conn, err := net.DialTimeout("unix", path, 150*time.Millisecond)
		if err == nil {
			_ = conn.SetWriteDeadline(time.Now().Add(time.Second))
			err = json.NewEncoder(conn).Encode(args)
			_ = conn.Close()
			return err == nil
		}
		time.Sleep(25 * time.Millisecond)
	}
	return false
}

func (i *instance) serve() {
	for {
		conn, err := i.listener.AcceptUnix()
		if err != nil {
			select {
			case <-i.done:
				return
			default:
			}
			continue
		}
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		var args []string
		if err := json.NewDecoder(conn).Decode(&args); err == nil {
			i.enqueue(args)
		}
		_ = conn.Close()
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

func (i *instance) close() {
	if i == nil {
		return
	}
	i.once.Do(func() {
		close(i.done)
		_ = i.listener.Close()
		_ = os.Remove(i.path)
	})
}
