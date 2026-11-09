//go:build ios

package apple

import (
	"errors"

	"github.com/ebitengine/purego/objc"
)

// holdSpeechSession makes the shared AVAudioSession one that records for
// dictation, as long as the session runs, then gives back what it was,
// letting the apps it interrupted play again.
func holdSpeechSession() (leave func(), err error) {
	shared := objc.ID(Class("AVAudioSession")).Send(sel("sharedInstance"))
	if shared == 0 {
		return nil, errors.New("apple: no audio session")
	}
	before := audioCategory{category: shared.Send(sel("category")).Send(sel("retain")),
		mode: shared.Send(sel("mode")).Send(sel("retain")), options: objc.Send[uint](shared, sel("categoryOptions"))}
	const allowBluetooth, defaultToSpeaker = 1 << 2, 1 << 3
	record := audioCategory{category: NSString("AVAudioSessionCategoryPlayAndRecord"),
		mode: NSString("AVAudioSessionModeMeasurement"), options: allowBluetooth | defaultToSpeaker}
	restore := func() {
		// AVAudioSessionSetActiveOptionNotifyOthersOnDeactivation is 1.
		objc.Send[bool](shared, sel("setActive:withOptions:error:"), false, uint(1), objc.ID(0))
		setAudioCategory(shared, before)
		releaseCategory(before)
	}
	if !setAudioCategory(shared, record) || !objc.Send[bool](shared, sel("setActive:error:"), true, objc.ID(0)) {
		restore()
		return nil, errors.New("apple: cannot activate the audio session for speech")
	}
	return restore, nil
}

// DeclaresUsage reports whether the app's Info.plist declares key, as iOS
// ends an app that asks for what it declares no reason for.
func DeclaresUsage(key string) bool {
	bundle := objc.ID(Class("NSBundle")).Send(sel("mainBundle"))
	return bundle.Send(sel("objectForInfoDictionaryKey:"), NSString(key)) != 0
}
