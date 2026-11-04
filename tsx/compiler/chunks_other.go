//go:build !js

package compiler

import "fmt"

// fetchChunk fails: only the web splits bundles, and other packages hold
// the whole app.
func fetchChunk(url string) (string, error) {
	return "", fmt.Errorf("%s: only web pages load chunks", url)
}
