// Package imageload fetches image bytes for the native backends: https
// and http, file paths and file: URIs, data: URIs, "asset:" paths under
// the app's resources and bytes the app holds. Requests for the same URI
// share one fetch, the bytes stay in a least-recently-used cache in
// memory, and downloads in a disk cache under HTTP's caching rules
// (UseDiskCache).
package imageload

import (
	"container/list"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/oneo-me/nsgo/internal/driver"
)

// ResourceRoot is the directory "asset:" paths resolve against: the
// package's resource root, Contents/Resources in a macOS bundle and the
// executable's directory elsewhere, where Windows and Linux packages put
// their resources. Android reads them from its APK instead.
var ResourceRoot = func() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	if runtime.GOOS == "darwin" {
		return filepath.Join(filepath.Dir(exe), "..", "Resources")
	}
	return filepath.Dir(exe)
}()

// ReadResource reads one of the app's resources by its slash-separated
// path, as "asset:" sources do.
func ReadResource(path string) ([]byte, error) { return readResource(path) }

// readResource reads one of the app's resources by its slash-separated
// path.
func readResource(path string) ([]byte, error) {
	if runtime.GOOS == "android" {
		return readAPKAsset(path)
	}
	return os.ReadFile(filepath.Join(ResourceRoot, filepath.FromSlash(path)))
}

// CacheBytes bounds the cache.
const CacheBytes = 32 << 20

// Progress receives bytes loaded and the total, 0 when unknown.
type Progress func(loaded, total int64)

var (
	mu       sync.Mutex
	inflight = map[string]*call{}
	cache    = newLRU(CacheBytes)
)

type call struct {
	done     chan struct{}
	data     []byte
	err      error
	waiters  int
	cancel   context.CancelFunc
	progress []Progress
}

// Load returns src's bytes. It blocks; call it on a goroutine of its own.
// Cancelling ctx stops waiting, and stops the fetch once nobody waits.
func Load(ctx context.Context, src driver.ImageSource, progress Progress) ([]byte, error) {
	if len(src.Data) > 0 {
		return src.Data, nil
	}
	uri := src.URI
	if uri == "" {
		return nil, errors.New("imageload: empty source")
	}
	mu.Lock()
	if data, ok := cache.get(uri); ok {
		mu.Unlock()
		return data, nil
	}
	c := inflight[uri]
	if c == nil {
		fetchCtx, cancel := context.WithCancel(context.Background())
		c = &call{done: make(chan struct{}), cancel: cancel}
		inflight[uri] = c
		go c.fetch(fetchCtx, uri)
	}
	c.waiters++
	if progress != nil {
		c.progress = append(c.progress, progress)
	}
	mu.Unlock()

	select {
	case <-c.done:
		return c.data, c.err
	case <-ctx.Done():
		mu.Lock()
		if c.waiters--; c.waiters == 0 {
			c.cancel()
		}
		mu.Unlock()
		return nil, ctx.Err()
	}
}

// Info answers an ItemImageInfo: it loads the item's source, which keeps
// its bytes in the cache for the views that show it next, and returns the
// EventImageInfo with the size read by size from the bytes, or the error.
// It blocks; call it on a goroutine of its own.
func Info(it *driver.MountItem, size func(data []byte) (driver.Size, error)) driver.Event {
	ev := driver.Event{Kind: driver.EventImageInfo, Count: it.Count}
	var src driver.ImageSource
	if it.Image != nil {
		src = it.Image.Source
	}
	data, err := Load(context.Background(), src, nil)
	if err == nil {
		ev.Size, err = size(data)
	}
	if err != nil {
		ev.Text = err.Error()
	}
	return ev
}

func (c *call) fetch(ctx context.Context, uri string) {
	data, err := fetch(ctx, uri, c.report)
	mu.Lock()
	delete(inflight, uri)
	if err == nil {
		cache.put(uri, data)
	}
	c.data, c.err = data, err
	mu.Unlock()
	close(c.done)
}

func (c *call) report(loaded, total int64) {
	mu.Lock()
	ps := append([]Progress(nil), c.progress...)
	mu.Unlock()
	for _, p := range ps {
		p(loaded, total)
	}
}

func fetch(ctx context.Context, uri string, progress Progress) ([]byte, error) {
	switch {
	case strings.HasPrefix(uri, "data:"):
		return decodeDataURI(uri)
	case strings.HasPrefix(uri, "asset:"):
		return readResource(strings.TrimLeft(uri[len("asset:"):], "/"))
	case strings.HasPrefix(uri, "file:"):
		u, err := url.Parse(uri)
		if err != nil {
			return nil, err
		}
		return os.ReadFile(filepath.FromSlash(u.Path))
	case strings.HasPrefix(uri, "http://"), strings.HasPrefix(uri, "https://"):
		if c := disk.Load(); c != nil {
			return c.fetch(ctx, uri, progress)
		}
		resp, err := get(ctx, uri, Header{}, progress)
		if err != nil {
			return nil, err
		}
		if resp.Status != 200 {
			return nil, fmt.Errorf("imageload: %s: status %d", uri, resp.Status)
		}
		return resp.Body, nil
	}
	return os.ReadFile(uri)
}

// ReadAll reads r to its end, total bytes long or 0 when unknown,
// calling progress as it goes: what an HTTP client reads a body with.
func ReadAll(r io.Reader, total int64, progress Progress) ([]byte, error) {
	buf := make([]byte, 0, max(total, 32<<10))
	for {
		if len(buf) == cap(buf) {
			buf = append(buf, 0)[:len(buf)]
		}
		n, err := r.Read(buf[len(buf):cap(buf)])
		buf = buf[:len(buf)+n]
		if n > 0 && progress != nil {
			progress(int64(len(buf)), max(total, 0))
		}
		if err == io.EOF {
			return buf, nil
		}
		if err != nil {
			return nil, err
		}
	}
}

func decodeDataURI(uri string) ([]byte, error) {
	meta, payload, ok := strings.Cut(uri[len("data:"):], ",")
	if !ok {
		return nil, errors.New("imageload: malformed data URI")
	}
	if strings.HasSuffix(meta, ";base64") {
		return base64.StdEncoding.DecodeString(payload)
	}
	s, err := url.PathUnescape(payload)
	return []byte(s), err
}

// lru is a byte-bounded least-recently-used cache.
type lru struct {
	limit, size int
	order       *list.List
	items       map[string]*list.Element
}

type entry struct {
	key  string
	data []byte
}

func newLRU(limit int) *lru {
	return &lru{limit: limit, order: list.New(), items: map[string]*list.Element{}}
}

func (c *lru) get(key string) ([]byte, bool) {
	if e, ok := c.items[key]; ok {
		c.order.MoveToFront(e)
		return e.Value.(*entry).data, true
	}
	return nil, false
}

func (c *lru) put(key string, data []byte) {
	if len(data) > c.limit {
		return
	}
	if e, ok := c.items[key]; ok {
		c.size -= len(e.Value.(*entry).data)
		c.order.Remove(e)
	}
	c.items[key] = c.order.PushFront(&entry{key, data})
	c.size += len(data)
	for c.size > c.limit {
		last := c.order.Back()
		ent := last.Value.(*entry)
		c.order.Remove(last)
		delete(c.items, ent.key)
		c.size -= len(ent.data)
	}
}
