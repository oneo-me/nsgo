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
	"syscall"
	"time"
)

// instance is what the first copy of an app holds: an exclusive lock on its lock file (instancePath), which ends with the process however it ends, and the Unix socket later launches hand their files to. Messages received before the GTK window exists wait in pending until Run installs the delivery function.
type instance struct {
	lock     *os.File
	listener *net.UnixListener
	path     string
	done     chan struct{}
	once     sync.Once

	mu      sync.Mutex
	deliver func([]string)
	pending [][]string
}

// claimInstance makes this process the owner, or hands its launch to the owner and reports that the caller should exit. The lock decides, as Windows' mutex does: whoever holds it owns the app ID, so two launches at once never both start an engine, and a socket a dead process left behind is replaced without asking it.
func claimInstance(id string, args []string) (*instance, bool) {
	path, err := instancePath(id)
	if err != nil {
		log.Printf("nsgo: single-instance setup: %v", err)
		return nil, false
	}
	lock, err := os.OpenFile(path+".lock", os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		log.Printf("nsgo: single-instance lock: %v", err)
		return nil, false
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		lock.Close()
		if !errors.Is(err, syscall.EWOULDBLOCK) {
			log.Printf("nsgo: single-instance lock: %v", err)
			return nil, false
		}
		if !sendInstance(path, args) {
			log.Printf("nsgo: could not hand the launch to the running instance")
		}
		return nil, true
	}
	i := &instance{lock: lock, path: path, done: make(chan struct{})}
	_ = os.Remove(path)
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		// The lock stays held, so later launches still find this owner, which hears none of them.
		log.Printf("nsgo: single-instance listener: %v", err)
		return i, false
	}
	_ = os.Chmod(path, 0o600)
	i.listener = listener
	go i.serve()
	return i, false
}

// instancePath is the socket of app id, beside its lock (the path with ".lock"), in a directory only this user may write and only this host sees: $XDG_RUNTIME_DIR, or a private directory in the temporary one. A directory another user could write would let them take the launches, and one on a network share, as a home directory can be, would let a copy on another host hold the lock. The name is a digest of id, as Linux limits a Unix socket's address to 108 bytes.
func instancePath(id string) (string, error) {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" || privateDir(dir) != nil {
		dir = filepath.Join(os.TempDir(), fmt.Sprintf("nsgo-%d", os.Getuid()))
		if err := os.Mkdir(dir, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
			return "", err
		}
		if err := privateDir(dir); err != nil {
			return "", err
		}
	}
	digest := sha256.Sum256([]byte(id))
	path := filepath.Join(dir, fmt.Sprintf("nsgo-%x.sock", digest[:12]))
	if len(path) >= 100 {
		return "", fmt.Errorf("%s: too long for a socket address", path)
	}
	return path, nil
}

// privateDir reports why dir is not a directory, not a symbolic link, of this user's that no one else may enter.
func privateDir(dir string) error {
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	switch {
	case !info.IsDir():
		return fmt.Errorf("%s: not a directory", dir)
	case !ok || int(st.Uid) != os.Getuid():
		return fmt.Errorf("%s: not owned by this user", dir)
	case info.Mode().Perm()&0o077 != 0:
		return fmt.Errorf("%s: open to other users (%v)", dir, info.Mode().Perm())
	}
	return nil
}

// sendInstance hands args to the owner, retrying while the owner, which already holds the lock, may not be listening yet.
func sendInstance(path string, args []string) bool {
	for range 20 {
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
		if i.listener != nil {
			_ = i.listener.Close()
			_ = os.Remove(i.path)
		}
		_ = i.lock.Close()
	})
}
