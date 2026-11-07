//go:build ios

package microphone

import (
	"errors"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

const (
	uikitPath       = "/System/Library/Frameworks/UIKit.framework/UIKit"
	foundationPath  = "/System/Library/Frameworks/AVFoundation.framework/AVFoundation"
	sessionPlayRec  = "AVAudioSessionCategoryPlayAndRecord"
	sessionMode     = "AVAudioSessionModeDefault"
	optionBluetooth = 1 << 2 // AVAudioSessionCategoryOptionAllowBluetooth
	optionSpeaker   = 1 << 3 // AVAudioSessionCategoryOptionDefaultToSpeaker
	// notifyOthers is AVAudioSessionSetActiveOptionNotifyOthersOnDeactivation.
	notifyOthers = 1
)

var errSession = errors.New("microphone: cannot activate the audio session")

// session is the shared AVAudioSession while recordings hold it: the
// category that also plays, so the app's sounds still do, and on leaving
// what it was before. Video sets the session too, on the main thread,
// and one that starts playing during a recording takes it away.
var session struct {
	sync.Mutex
	users          int
	category, mode objc.ID
	options        uint
}

func sel(name string) objc.SEL { return objc.RegisterName(name) }

func audioSession() objc.ID {
	return objc.ID(objc.GetClass("AVAudioSession")).Send(sel("sharedInstance"))
}

// global reads the object a framework exports under name, 0 if it has none.
func global(path, name string) objc.ID {
	lib, err := purego.Dlopen(path, purego.RTLD_NOW)
	if err != nil {
		return 0
	}
	symbol, err := purego.Dlsym(lib, name)
	if err != nil {
		return 0
	}
	return **(**objc.ID)(unsafe.Pointer(&symbol))
}

// holdSession makes the session one that records, for the first user.
func holdSession() error {
	session.Lock()
	defer session.Unlock()
	if session.users > 0 {
		session.users++
		return nil
	}
	shared := audioSession()
	category, mode := global(foundationPath, sessionPlayRec), global(foundationPath, sessionMode)
	if shared == 0 || category == 0 || mode == 0 {
		return errSession
	}
	session.category = shared.Send(sel("category")).Send(sel("retain"))
	session.mode = shared.Send(sel("mode")).Send(sel("retain"))
	session.options = objc.Send[uint](shared, sel("categoryOptions"))
	if !objc.Send[bool](shared, sel("setCategory:mode:options:error:"), category, mode, uint(optionBluetooth|optionSpeaker), objc.ID(0)) ||
		!objc.Send[bool](shared, sel("setActive:error:"), true, objc.ID(0)) {
		restoreSession(shared)
		return errSession
	}
	session.users = 1
	return nil
}

// releaseSession gives the session back once its last user left, letting
// the apps recording interrupted play again.
func releaseSession() {
	session.Lock()
	defer session.Unlock()
	if session.users--; session.users > 0 {
		return
	}
	shared := audioSession()
	objc.Send[bool](shared, sel("setActive:withOptions:error:"), false, uint(notifyOthers), objc.ID(0))
	restoreSession(shared)
}

func restoreSession(shared objc.ID) {
	objc.Send[bool](shared, sel("setCategory:mode:options:error:"), session.category, session.mode, session.options, objc.ID(0))
	session.category.Send(sel("release"))
	session.mode.Send(sel("release"))
}

// enterSession holds the session, calling ended if an interruption or the
// app leaving the screen stops the recording. Its leave deactivates off
// the caller's thread, as that waits for the system.
func enterSession(ended func()) (leave func(), err error) {
	if err := holdSession(); err != nil {
		return nil, err
	}
	center := objc.ID(objc.GetClass("NSNotificationCenter")).Send(sel("defaultCenter"))
	block := objc.NewBlock(func(objc.Block, objc.ID) { ended() })
	var tokens []objc.ID
	for _, name := range []objc.ID{global(foundationPath, "AVAudioSessionInterruptionNotification"),
		global(uikitPath, "UIApplicationDidEnterBackgroundNotification")} {
		token := center.Send(sel("addObserverForName:object:queue:usingBlock:"), name, objc.ID(0), objc.ID(0), block)
		tokens = append(tokens, token.Send(sel("retain")))
	}
	block.Release()
	return func() {
		for _, token := range tokens {
			center.Send(sel("removeObserver:"), token)
			token.Send(sel("release"))
		}
		go releaseSession()
	}, nil
}

// checkUsageDescription stands in front of the permission prompt, as iOS
// aborts an app that declares no usage description.
func checkUsageDescription() error {
	bundle := objc.ID(objc.GetClass("NSBundle")).Send(sel("mainBundle"))
	key := objc.ID(objc.GetClass("NSString")).Send(sel("stringWithUTF8String:"), unsafe.StringData("NSMicrophoneUsageDescription\x00"))
	if bundle.Send(sel("objectForInfoDictionaryKey:"), key) == 0 {
		return ErrNoUsageDescription
	}
	return nil
}
