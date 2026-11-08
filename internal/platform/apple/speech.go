//go:build darwin

package apple

import (
	"log"
	"sync"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/driver"
)

// Speech recognizes speech for macOS and iOS, as NSH's speech.rs: the
// Speech framework's recognizer fed by AVAudioEngine's input, one session
// in the process at a time, as there is one microphone. Its state is the
// main thread's; the recognizer's handler and the permission prompts hop
// there.
var speech speechState

type speechState struct{ run *speechRun }

// speechRun is a session's objects, each retained.
type speechRun struct {
	id                    uint32
	dispatch              func(driver.Event)
	interim               bool
	engine, request, task objc.ID
	recognizer            objc.ID
	leave                 func()
	text                  string
	done                  bool
}

const (
	speechPath    = "/System/Library/Frameworks/Speech.framework/Speech"
	avPath        = "/System/Library/Frameworks/AVFoundation.framework/AVFoundation"
	authNotAsked  = 0
	authAllowed   = 3 // SFSpeechRecognizerAuthorizationStatusAuthorized, AVAuthorizationStatusAuthorized
	speechBufSize = 1024
)

// SpeechAvailable is Environment.Speech: every macOS and iOS NSGo runs on
// has the Speech framework, which loads only once a session starts.
func SpeechAvailable() bool { return true }

// loadSpeech loads the Speech framework and AVFoundation, reporting
// whether they have what a session needs.
var loadSpeech = sync.OnceValue(func() bool {
	if _, err := purego.Dlopen(speechPath, purego.RTLD_NOW|purego.RTLD_GLOBAL); err != nil {
		return false
	}
	if _, err := purego.Dlopen(avPath, purego.RTLD_NOW|purego.RTLD_GLOBAL); err != nil {
		return false
	}
	return objc.GetClass("SFSpeechRecognizer") != 0 && objc.GetClass("AVAudioEngine") != 0
})

// HandleSpeech carries out an ItemSpeech for session id, on the main
// thread, reporting EventSpeech through dispatch.
func HandleSpeech(id uint32, r driver.SpeechRequest, dispatch func(driver.Event)) {
	s := &speech
	switch r.Command {
	case driver.SpeechStart:
		if s.run != nil {
			s.teardown(s.run)
		}
		run := &speechRun{id: id, dispatch: dispatch, interim: r.Interim}
		s.run = run
		if !loadSpeech() {
			s.finish(run, driver.SpeechUnavailable)
			return
		}
		for _, key := range []string{"NSMicrophoneUsageDescription", "NSSpeechRecognitionUsageDescription"} {
			if !DeclaresUsage(key) {
				// The system would end the app as it asks.
				log.Printf("nsgo: speech needs %s in the app's Info.plist (nsgo.yaml's apple.info)", key)
				s.finish(run, driver.SpeechNotAllowed)
				return
			}
		}
		s.authorize(run, func() { s.begin(run, r.Lang) })
	case driver.SpeechStop:
		run := s.run
		switch {
		case run == nil || run.id != id:
		case run.engine == 0:
			// Still asking for permission: it ends with nothing heard.
			s.finish(run, "")
		default:
			// The recognizer finishes with what it heard: a final result,
			// or an error when it heard nothing.
			s.stopAudio(run)
			run.request.Send(sel("endAudio"))
		}
	case driver.SpeechAbort:
		if run := s.run; run != nil && run.id == id {
			run.done = true
			s.teardown(run)
		}
	}
}

// authorize has the user allow speech recognition and the microphone,
// asking once each, then runs next; a refusal ends the session.
func (s *speechState) authorize(run *speechRun, next func()) {
	recognizer := objc.ID(objc.GetClass("SFSpeechRecognizer"))
	switch objc.Send[int](recognizer, sel("authorizationStatus")) {
	case authAllowed:
	case authNotAsked:
		block := objc.NewBlock(func(_ objc.Block, status int) {
			OnMain(func() {
				if status != authAllowed {
					s.finish(run, driver.SpeechNotAllowed)
					return
				}
				s.authorize(run, next)
			})
		})
		recognizer.Send(sel("requestAuthorization:"), block)
		block.Release()
		return
	default:
		s.finish(run, driver.SpeechNotAllowed)
		return
	}
	device, media := objc.ID(objc.GetClass("AVCaptureDevice")), Global(avPath, "AVMediaTypeAudio")
	switch objc.Send[int](device, sel("authorizationStatusForMediaType:"), media) {
	case authAllowed:
		if !run.done {
			next()
		}
	case authNotAsked:
		block := objc.NewBlock(func(_ objc.Block, granted bool) {
			OnMain(func() {
				if !granted {
					s.finish(run, driver.SpeechNotAllowed)
					return
				}
				if !run.done {
					next()
				}
			})
		})
		device.Send(sel("requestAccessForMediaType:completionHandler:"), media, block)
		block.Release()
	default:
		s.finish(run, driver.SpeechNotAllowed)
	}
}

// begin starts the recognizer of lang on the microphone's input.
func (s *speechState) begin(run *speechRun, lang string) {
	if run.done || s.run != run {
		return
	}
	alloc := objc.ID(objc.GetClass("SFSpeechRecognizer")).Send(sel("alloc"))
	if lang != "" {
		locale := objc.ID(objc.GetClass("NSLocale")).Send(sel("localeWithLocaleIdentifier:"), NSString(lang))
		run.recognizer = alloc.Send(sel("initWithLocale:"), locale)
	} else {
		run.recognizer = alloc.Send(sel("init"))
	}
	if run.recognizer == 0 {
		s.finish(run, driver.SpeechLanguageUnsupported)
		return
	}
	if !objc.Send[bool](run.recognizer, sel("isAvailable")) {
		s.finish(run, driver.SpeechNetwork)
		return
	}
	leave, err := holdSpeechSession()
	if err != nil {
		s.finish(run, driver.SpeechAudioCapture)
		return
	}
	run.leave = leave
	run.engine = objc.ID(objc.GetClass("AVAudioEngine")).Send(sel("new"))
	input := run.engine.Send(sel("inputNode"))
	format := input.Send(sel("outputFormatForBus:"), uint(0))
	if input == 0 || format == 0 || objc.Send[float64](format, sel("sampleRate")) == 0 {
		s.finish(run, driver.SpeechAudioCapture)
		return
	}
	run.request = objc.ID(objc.GetClass("SFSpeechAudioBufferRecognitionRequest")).Send(sel("new"))
	run.request.Send(sel("setShouldReportPartialResults:"), run.interim)
	request := run.request
	// On the audio thread: the request takes buffers from any thread.
	tap := objc.NewBlock(func(_ objc.Block, buffer, _ objc.ID) { request.Send(sel("appendAudioPCMBuffer:"), buffer) })
	input.Send(sel("installTapOnBus:bufferSize:format:block:"), uint(0), uint32(speechBufSize), format, tap)
	tap.Release()
	run.engine.Send(sel("prepare"))
	if !objc.Send[bool](run.engine, sel("startAndReturnError:"), objc.ID(0)) {
		s.finish(run, driver.SpeechAudioCapture)
		return
	}
	handler := objc.NewBlock(func(_ objc.Block, result, failure objc.ID) {
		// Read what the objects say here, as they are gone by the time
		// the main thread runs.
		var text, code string
		final := false
		if result != 0 {
			text = GoString(result.Send(sel("bestTranscription")).Send(sel("formattedString")))
			final = objc.Send[bool](result, sel("isFinal"))
		} else if failure != 0 {
			code = speechError(GoString(failure.Send(sel("domain"))), objc.Send[int](failure, sel("code")))
		}
		OnMain(func() {
			if run.done || s.run != run {
				return
			}
			if result != 0 {
				run.text = text
				s.report(run, driver.SpeechEvent{Phase: driver.SpeechResult, Text: text, Final: final})
				if final {
					s.finish(run, "")
				}
				return
			}
			s.finish(run, code)
		})
	})
	run.task = run.recognizer.Send(sel("recognitionTaskWithRequest:resultHandler:"), run.request, handler).Send(sel("retain"))
	handler.Release()
	s.report(run, driver.SpeechEvent{Phase: driver.SpeechStarted})
}

// speechError names an error of the recognizer as the Web Speech API
// does, "" for a request that was cancelled.
func speechError(domain string, code int) string {
	switch domain {
	case "kAFAssistantErrorDomain":
		switch code {
		case 203, 1110: // nothing recognized, no speech detected
			return driver.SpeechNoSpeech
		case 216, 209: // cancelled
			return ""
		case 1700: // dictation is off
			return driver.SpeechNotAllowed
		}
	case "kLSRErrorDomain":
		switch code {
		case 301: // cancelled
			return ""
		case 102: // the language's assets are missing
			return driver.SpeechLanguageUnsupported
		case 201: // Siri and Dictation are off
			return driver.SpeechNotAllowed
		}
	case "NSURLErrorDomain":
		return driver.SpeechNetwork
	}
	return driver.SpeechFailed
}

func (s *speechState) report(run *speechRun, e driver.SpeechEvent) {
	run.dispatch(driver.Event{Kind: driver.EventSpeech, Count: run.id, Data: &e})
}

// finish ends the session with code, "" for none, once.
func (s *speechState) finish(run *speechRun, code string) {
	if run.done {
		return
	}
	run.done = true
	s.teardown(run)
	s.report(run, driver.SpeechEvent{Phase: driver.SpeechEnded, Text: run.text, Error: code})
}

// stopAudio stops the microphone, once.
func (s *speechState) stopAudio(run *speechRun) {
	if run.engine == 0 {
		return
	}
	run.engine.Send(sel("stop"))
	run.engine.Send(sel("inputNode")).Send(sel("removeTapOnBus:"), uint(0))
	run.engine.Send(sel("release"))
	run.engine = 0
	if run.leave != nil {
		run.leave()
		run.leave = nil
	}
}

// teardown lets the session's objects go, cancelling the recognizer.
func (s *speechState) teardown(run *speechRun) {
	s.stopAudio(run)
	if run.task != 0 {
		run.task.Send(sel("cancel"))
		run.task.Send(sel("release"))
		run.task = 0
	}
	for _, o := range []*objc.ID{&run.request, &run.recognizer} {
		if *o != 0 {
			(*o).Send(sel("release"))
			*o = 0
		}
	}
	if s.run == run {
		s.run = nil
	}
}
