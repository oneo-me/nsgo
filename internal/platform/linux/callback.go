//go:build linux && !android

package linux

// purego keeps every NewCallback trampoline for the process's life, so
// the backend makes one per callback shared by every window, view and
// menu item, rather than one per connection. Menu items and entries are
// rebuilt as an app changes sections, and per-connection trampolines
// would exhaust purego's limit.
var (
	cbCloseRequest uintptr
	cbApply        uintptr
	cbRun          uintptr
	cbWindowState  uintptr
	cbMapped       uintptr
	cbActivity     uintptr

	cbClickPressed     uintptr
	cbClickReleased    uintptr
	cbMotion           uintptr
	cbLeave            uintptr
	cbDragBegin        uintptr
	cbDragUpdate       uintptr
	cbDragEnd          uintptr
	cbZoomChanged      uintptr
	cbRotateChanged    uintptr
	cbGestureEnd       uintptr
	cbScroll           uintptr
	cbScrollBegin      uintptr
	cbScrollDecelerate uintptr
	cbKeyPressed       uintptr
	cbKeyReleased      uintptr
	cbModifiers        uintptr
	cbPinchTimeout     uintptr

	cbEntryChanged  uintptr
	cbEntryActivate uintptr
	cbEntryFocus    uintptr
	cbAlertChosen   uintptr

	cbWebLoadChanged uintptr
	cbWebLoadFailed  uintptr
	cbWebPolicy      uintptr
	cbWebMessage     uintptr
	cbWebMap         uintptr
	cbWebKey         uintptr
	cbWebButton      uintptr
	cbWebMotion      uintptr
	cbWebScroll      uintptr

	cbShield           uintptr
	cbViewClassInit    uintptr
	cbViewDispose      uintptr
	cbViewSizeAllocate uintptr
	cbViewSnapshot     uintptr
	cbViewContains     uintptr
	cbVideoPrepared    uintptr
	cbVideoSize        uintptr
	cbVideoEnded       uintptr
	cbVideoError       uintptr
	cbVideoProgress    uintptr

	cbSettingsChanged uintptr
	cbMenuActivate    uintptr
	cbTick            uintptr

	cbShaderRender    uintptr
	cbShaderUnrealize uintptr

	cbDropMotion uintptr
	cbDropLeave  uintptr
	cbDrop       uintptr
)

func init() {
	cbCloseRequest = newCallback(closeRequestCallback)
	cbApply = newCallback(applyCallback)
	cbRun = newCallback(runCallback)
	cbWindowState = newCallback(windowStateCallback)
	cbMapped = newCallback(mappedCallback)
	cbActivity = newCallback(activityCallback)

	cbClickPressed = newCallback(clickPressedCallback)
	cbClickReleased = newCallback(clickReleasedCallback)
	cbMotion = newCallback(motionCallback)
	cbLeave = newCallback(leaveCallback)
	cbDragBegin = newCallback(dragBeginCallback)
	cbDragUpdate = newCallback(dragUpdateCallback)
	cbDragEnd = newCallback(dragEndCallback)
	cbZoomChanged = newCallback(zoomChangedCallback)
	cbRotateChanged = newCallback(rotateChangedCallback)
	cbGestureEnd = newCallback(gestureEndCallback)
	cbScroll = newCallback(scrollCallback)
	cbScrollBegin = newCallback(scrollBeginCallback)
	cbScrollDecelerate = newCallback(scrollDecelerateCallback)
	cbKeyPressed = newCallback(keyPressedCallback)
	cbKeyReleased = newCallback(keyReleasedCallback)
	cbModifiers = newCallback(modifiersCallback)
	cbPinchTimeout = newCallback(pinchTimeoutCallback)

	cbEntryChanged = newCallback(entryChangedCallback)
	cbEntryActivate = newCallback(entryActivateCallback)
	cbEntryFocus = newCallback(entryFocusCallback)
	cbAlertChosen = newCallback(alertChosenCallback)

	cbWebLoadChanged = newCallback(webLoadChangedCallback)
	cbWebLoadFailed = newCallback(webLoadFailedCallback)
	cbWebPolicy = newCallback(webPolicyCallback)
	cbWebMessage = newCallback(webMessageCallback)
	cbWebMap = newCallback(webMapCallback)
	cbWebKey = newCallback(webKeyCallback)
	cbWebButton = newCallback(webButtonCallback)
	cbWebMotion = newCallback(webMotionCallback)
	cbWebScroll = newCallback(webScrollCallback)

	cbShield = newCallback(shieldCallback)
	cbViewClassInit = newCallback(viewClassInitCallback)
	cbViewDispose = newCallback(viewDisposeCallback)
	cbViewSizeAllocate = newCallback(viewSizeAllocateCallback)
	cbViewSnapshot = newCallback(viewSnapshotCallback)
	cbViewContains = newCallback(viewContainsCallback)
	cbVideoPrepared = newCallback(videoPreparedCallback)
	cbVideoSize = newCallback(videoSizeCallback)
	cbVideoEnded = newCallback(videoEndedCallback)
	cbVideoError = newCallback(videoErrorCallback)
	cbVideoProgress = newCallback(videoProgressCallback)

	cbSettingsChanged = newCallback(settingsChangedCallback)
	cbMenuActivate = newCallback(menuActivateCallback)
	cbTick = newCallback(tickCallback)

	cbShaderRender = newCallback(shaderRenderCallback)
	cbShaderUnrealize = newCallback(shaderUnrealizeCallback)

	cbDropMotion = newCallback(dropMotionCallback)
	cbDropLeave = newCallback(dropLeaveCallback)
	cbDrop = newCallback(dropCallback)
}
