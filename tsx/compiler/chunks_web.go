//go:build js

package compiler

import (
	"fmt"
	"os"
	"syscall/js"
)

// fetchChunk loads a split bundle's chunk from url, relative to the page:
// the chunk's own file runs it in the app's realm, so a page needs no
// 'unsafe-eval', and the runtime finds what it defines by the chunk's
// name (docs/architecture/tsx.md).
func fetchChunk(url string) (string, error) {
	if url == "" {
		return "", fmt.Errorf("no such chunk")
	}
	name := os.Getenv("NSGO_REALM")
	if name == "" {
		return "", fmt.Errorf("the page has no realm for %s; serve the app with the loader nsgo build -target web writes", url)
	}
	abs := js.Global().Get("URL").New(url, js.Global().Get("document").Get("baseURI")).Get("href").String()
	return "", loadScript(js.Global().Get(name), abs)
}

// loadScript runs the classic script at url in frame's document, and
// returns once it ran.
func loadScript(frame js.Value, url string) error {
	doc := frame.Get("contentWindow").Get("document")
	script := doc.Call("createElement", "script")
	script.Set("src", url)
	done := make(chan error, 1)
	loaded := js.FuncOf(func(js.Value, []js.Value) any {
		done <- nil
		return nil
	})
	failed := js.FuncOf(func(js.Value, []js.Value) any {
		done <- fmt.Errorf("%s could not be loaded", url)
		return nil
	})
	defer loaded.Release()
	defer failed.Release()
	script.Set("onload", loaded)
	script.Set("onerror", failed)
	doc.Get("head").Call("append", script)
	return <-done
}
