//go:build ios

package apple

import (
	"log"
	"slices"

	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// The shared AVAudioSession follows the videos that play with a session
// of their own, as NSH's iOS backend does: while one that does not mix
// plays it is the playback category in the spoken audio mode, else the
// same mixing with others; once none plays it goes back to what it was
// and is deactivated, telling the apps it interrupted that they may play
// again. Only the main thread touches it here; microphone holds it too
// while it records.

// audioCategory is a session's category, mode and options.
type audioCategory struct {
	category, mode objc.ID
	options        uint
}

var audio struct {
	playing []audioPlay
	applied driver.AudioSession // AudioSessionDefault: none
	before  audioCategory
}

type audioPlay struct {
	key     *Video
	session driver.AudioSession
}

// audioPlaying says video key plays with session, or plays no more when
// session is AudioSessionDefault.
func audioPlaying(key *Video, session driver.AudioSession) {
	audio.playing = slices.DeleteFunc(audio.playing, func(p audioPlay) bool { return p.key == key })
	if session != driver.AudioSessionDefault {
		audio.playing = append(audio.playing, audioPlay{key, session})
	}
	wanted := driver.AudioSessionDefault
	for _, p := range audio.playing {
		if wanted != driver.AudioSessionPlayback {
			wanted = p.session
		}
	}
	if wanted == audio.applied {
		return
	}
	shared := objc.ID(Class("AVAudioSession")).Send(sel("sharedInstance"))
	if shared == 0 {
		return
	}
	if wanted == driver.AudioSessionDefault {
		// AVAudioSessionSetActiveOptionNotifyOthersOnDeactivation is 1.
		objc.Send[bool](shared, sel("setActive:withOptions:error:"), false, uint(1), objc.ID(0))
		if !setAudioCategory(shared, audio.before) {
			log.Print("nsgo: cannot restore the audio session after a video")
		}
		releaseCategory(audio.before)
		audio.applied = driver.AudioSessionDefault
		return
	}
	if audio.applied == driver.AudioSessionDefault {
		audio.before = audioCategory{category: shared.Send(sel("category")).Send(sel("retain")),
			mode: shared.Send(sel("mode")).Send(sel("retain")), options: objc.Send[uint](shared, sel("categoryOptions"))}
	}
	var options uint
	if wanted == driver.AudioSessionMixWithOthers {
		options = 1 // AVAudioSessionCategoryOptionMixWithOthers
	}
	c := audioCategory{category: NSString("AVAudioSessionCategoryPlayback"), mode: NSString("AVAudioSessionModeSpokenAudio"), options: options}
	if !setAudioCategory(shared, c) || !objc.Send[bool](shared, sel("setActive:error:"), true, objc.ID(0)) {
		log.Print("nsgo: cannot set and activate the audio session for a video")
	}
	audio.applied = wanted
}

func setAudioCategory(shared objc.ID, c audioCategory) bool {
	if c.category == 0 {
		return false
	}
	return objc.Send[bool](shared, sel("setCategory:mode:options:error:"), c.category, c.mode, c.options, objc.ID(0))
}

func releaseCategory(c audioCategory) {
	for _, id := range []objc.ID{c.category, c.mode} {
		if id != 0 {
			id.Send(sel("release"))
		}
	}
}
