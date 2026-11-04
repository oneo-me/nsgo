//go:build js

package compiler

import (
	"fmt"
	"io"
	"net/http"
	"syscall/js"
)

// fetchChunk fetches a split bundle's chunk from url, relative to the
// page.
func fetchChunk(url string) (string, error) {
	if url == "" {
		return "", fmt.Errorf("no such chunk")
	}
	abs := js.Global().Get("URL").New(url, js.Global().Get("document").Get("baseURI")).Get("href").String()
	res, err := http.Get(abs)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s: %s", url, res.Status)
	}
	b, err := io.ReadAll(res.Body)
	return string(b), err
}
