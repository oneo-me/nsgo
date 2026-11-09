//go:build !darwin

package widget

func containerDir() (string, error) { return "", ErrUnsupported }
func reload(string) error           { return nil }
