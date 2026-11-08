package core

import (
	"fmt"

	"github.com/oneo-me/nsgo/internal/driver"
)

// SpeechError is why a speech session ended without its transcript,
// named as the Web Speech API's errors.
type SpeechError string

const (
	// SpeechNotAllowed: the user or the system refused the microphone or
	// the recognizer.
	SpeechNotAllowed SpeechError = driver.SpeechNotAllowed
	// SpeechNoSpeech: nothing was said.
	SpeechNoSpeech SpeechError = driver.SpeechNoSpeech
	// SpeechAudioCapture: the microphone could not record.
	SpeechAudioCapture SpeechError = driver.SpeechAudioCapture
	// SpeechNetwork: the recognizer needed the network.
	SpeechNetwork SpeechError = driver.SpeechNetwork
	// SpeechLanguageUnsupported: the recognizer has no model for the
	// language.
	SpeechLanguageUnsupported SpeechError = driver.SpeechLanguageUnsupported
	// SpeechUnavailable: this platform recognizes no speech
	// (Env.CanRecognizeSpeech).
	SpeechUnavailable SpeechError = driver.SpeechUnavailable
	// SpeechFailed: the recognizer failed otherwise.
	SpeechFailed SpeechError = driver.SpeechFailed
)

func (e SpeechError) Error() string { return "nsgo: speech: " + string(e) }

// SpeechOptions are a speech session's, as NSH's Speech.start takes them.
type SpeechOptions struct {
	// Lang is the BCP 47 tag of the language spoken; "" is the app's
	// locale.
	Lang string
	// Interim reports results before they are final.
	Interim bool
	// OnStart runs once the recognizer listens.
	OnStart func()
	// OnResult runs with the whole transcript so far, and whether the
	// recognizer will not change it.
	OnResult func(text string, final bool)
	// OnEnd runs once, last, with the last transcript and why it ended
	// early: a SpeechError, nil when it finished or was stopped or
	// aborted.
	OnEnd func(text string, err error)
}

// SpeechSession is speech being recognized, from the microphone.
type SpeechSession struct {
	rt    *runtime
	tree  *Tree
	owner *Scope
	id    uint32
	opts  SpeechOptions
	text  string
	ended bool
}

// StartSpeech recognizes what the user says, one session at a time: a
// new one aborts the one running, and the scope that started it going
// aborts it too. macOS and iOS recognize with the Speech framework, the
// app's Info.plist declaring NSMicrophoneUsageDescription and
// NSSpeechRecognitionUsageDescription; elsewhere the session ends with
// SpeechUnavailable. It fails only for a Lang that is no language tag.
func (cx *Scope) StartSpeech(opts SpeechOptions) (*SpeechSession, error) {
	if opts.Lang != "" && !languageTag(opts.Lang) {
		return nil, fmt.Errorf("nsgo: speech: %q is no language tag", opts.Lang)
	}
	rt, t := cx.rt, cx.tree()
	if t == nil {
		return nil, SpeechUnavailable
	}
	if rt.speech != nil {
		rt.speech.Abort()
	}
	t.alertID++
	s := &SpeechSession{rt: rt, tree: t, owner: cx.current(), id: t.alertID, opts: opts}
	rt.speech = s
	s.owner.addCleanup(s.Abort)
	if !Untracked(cx, cx.Env().CanRecognizeSpeech) {
		cx.Post(func() { s.end(driver.SpeechEvent{Phase: driver.SpeechEnded, Error: driver.SpeechUnavailable}) })
		return s, nil
	}
	lang := opts.Lang
	if lang == "" {
		lang = Untracked(cx, cx.Env().Locale)
	}
	s.send(driver.SpeechRequest{Command: driver.SpeechStart, Lang: lang, Interim: opts.Interim})
	return s, nil
}

// Text returns the transcript so far.
func (s *SpeechSession) Text() string { return s.text }

// Ended reports whether the session ended.
func (s *SpeechSession) Ended() bool { return s.ended }

// Stop has the recognizer finish what it heard: a last final result,
// then the end.
func (s *SpeechSession) Stop() {
	if !s.ended {
		s.send(driver.SpeechRequest{Command: driver.SpeechStop})
	}
}

// Abort ends the session at once with the transcript so far.
func (s *SpeechSession) Abort() {
	if s.ended {
		return
	}
	s.send(driver.SpeechRequest{Command: driver.SpeechAbort})
	s.end(driver.SpeechEvent{Phase: driver.SpeechEnded, Text: s.text})
}

func (s *SpeechSession) send(r driver.SpeechRequest) {
	s.tree.commands = append(s.tree.commands, driver.MountItem{Kind: driver.ItemSpeech, Count: s.id, Speech: &r})
	s.tree.changed = true
}

// speechEvent hands a platform's report to the session it is of.
func (rt *runtime) speechEvent(ev driver.Event) {
	s := rt.speech
	report, _ := ev.Data.(*driver.SpeechEvent)
	if s == nil || s.id != ev.Count || report == nil || s.ended {
		return
	}
	switch report.Phase {
	case driver.SpeechStarted:
		if s.opts.OnStart != nil {
			s.within(s.opts.OnStart)
		}
	case driver.SpeechResult:
		s.text = report.Text
		if s.opts.OnResult != nil {
			s.within(func() { s.opts.OnResult(report.Text, report.Final) })
		}
	case driver.SpeechEnded:
		s.end(*report)
	}
}

// end ends the session, once.
func (s *SpeechSession) end(report driver.SpeechEvent) {
	if s.ended {
		return
	}
	s.ended = true
	if s.rt.speech == s {
		s.rt.speech = nil
	}
	if report.Text != "" {
		s.text = report.Text
	}
	var err error
	if report.Error != "" {
		err = SpeechError(report.Error)
	}
	if s.opts.OnEnd != nil {
		s.within(func() { s.opts.OnEnd(s.text, err) })
	}
}

// within runs fn under the session's owner, or the root once it is gone.
func (s *SpeechSession) within(fn func()) {
	owner := s.owner
	if owner.disposed {
		owner = s.tree.root
	}
	s.tree.within(owner, fn)
}

// languageTag reports whether tag looks like a BCP 47 tag: subtags of 1
// to 8 letters and digits, the first of letters.
func languageTag(tag string) bool {
	first := true
	n := 0
	for i := 0; i <= len(tag); i++ {
		if i == len(tag) || tag[i] == '-' || tag[i] == '_' {
			if n == 0 || n > 8 {
				return false
			}
			first, n = false, 0
			continue
		}
		c := tag[i]
		letter := c|0x20 >= 'a' && c|0x20 <= 'z'
		if !letter && (first || c < '0' || c > '9') {
			return false
		}
		n++
	}
	return true
}
