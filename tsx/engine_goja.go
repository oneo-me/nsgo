//go:build !js

package tsx

// attachEngine has nothing to do on goja, which runs promise reactions as
// the outermost call into JS returns.
func (r *runtime) attachEngine() {}
