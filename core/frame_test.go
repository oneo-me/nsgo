package core

import "testing"

// An app that registers a frame callback must make the host run frames, and
// must stop doing so the moment nothing is registered: a static screen that
// keeps asking for frames is exactly the wakeup cost this exists to avoid.
func TestOnFrameRunsUntilFalse(t *testing.T) {
	var animate []bool
	root := NewRoot(RootOptions{
		Post:    func(func()) {},
		Animate: func(on bool) { animate = append(animate, on) },
	})

	runs := 0
	root.OnFrame(func() bool {
		runs++
		return runs < 3
	})
	if !root.WantsFrames() {
		t.Fatal("registering a frame callback did not ask for frames")
	}
	if len(animate) != 1 || !animate[0] {
		t.Fatalf("the host was told %v, want one true", animate)
	}

	for range 5 {
		root.RunFrames()
	}
	if runs != 3 {
		t.Fatalf("the callback ran %d times, want 3", runs)
	}
	if root.WantsFrames() {
		t.Fatal("still wants frames after the callback returned false")
	}
	if len(animate) != 2 || animate[1] {
		t.Fatalf("the host was told %v, want a false last", animate)
	}
}

// Stopping a callback by hand takes it out too.
func TestOnFrameStop(t *testing.T) {
	root := NewRoot(RootOptions{Post: func(func()) {}})
	runs := 0
	stop := root.OnFrame(func() bool { runs++; return true })

	root.RunFrames()
	stop()
	root.RunFrames()
	if runs != 1 {
		t.Fatalf("the callback ran %d times after being stopped, want 1", runs)
	}
	if root.WantsFrames() {
		t.Fatal("stopping the last callback left frames wanted")
	}
}

// Two callbacks: frames keep running while either wants them.
func TestOnFrameKeepsRunningForTheOther(t *testing.T) {
	root := NewRoot(RootOptions{Post: func(func()) {}})
	a, b := 0, 0
	root.OnFrame(func() bool { a++; return a < 2 })
	root.OnFrame(func() bool { b++; return true })

	for range 4 {
		root.RunFrames()
	}
	if a != 2 || b != 4 {
		t.Fatalf("callbacks ran %d and %d, want 2 and 4", a, b)
	}
	if !root.WantsFrames() {
		t.Fatal("frames stopped while one callback still wants them")
	}
}
