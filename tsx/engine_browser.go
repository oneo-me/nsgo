//go:build js

package tsx

// attachEngine runs what the page's engine calls of Go outside a call
// from Go, as a promise's reactions and the browser's events do, under
// the app's root, untracked, and commits it after; the realm goes with
// the app.
func (r *runtime) attachEngine() {
	r.vm.SetEntry(func(run func()) {
		r.cx.Run(run)
		if !r.committing {
			r.committing = true
			r.cx.Post(func() { r.committing = false })
		}
	})
	r.cx.Cleanup(r.vm.Close)
}
