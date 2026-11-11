//go:build darwin && !ios

package macos

// CoreFoundation loads with the executable, rather than with AppKit when
// the app starts: its initializer has libsecinit enter the App Sandbox,
// and a sandboxed process that has not, as a Go executable loading
// AppKit alone would be, is denied Launch Services and aborts in
// NSApplication. Mac App Store builds run sandboxed (macstore.go).
//go:cgo_import_dynamic _ _ "/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation"
