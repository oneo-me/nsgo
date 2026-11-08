package driver

// SpeechCommand is what an ItemSpeech asks of its session.
type SpeechCommand uint8

const (
	// SpeechStart starts recognizing in Lang, the system's language when
	// empty.
	SpeechStart SpeechCommand = iota
	// SpeechStop has the recognizer finish: a last final result, then
	// the end.
	SpeechStop
	// SpeechAbort ends the session at once, reporting nothing more.
	SpeechAbort
)

// SpeechRequest is an ItemSpeech's request.
type SpeechRequest struct {
	Command SpeechCommand
	// Lang is a BCP 47 tag for SpeechStart.
	Lang string
	// Interim reports results before they are final.
	Interim bool
}

// SpeechPhase is what an EventSpeech reports of its session.
type SpeechPhase uint8

const (
	// SpeechStarted says the recognizer listens.
	SpeechStarted SpeechPhase = iota
	// SpeechResult carries the whole transcript so far.
	SpeechResult
	// SpeechEnded ends the session with the last transcript, and Error
	// when it failed. A platform reports it once, last.
	SpeechEnded
)

// The errors a session ends with, named as the Web Speech API's.
const (
	SpeechNotAllowed          = "not-allowed"
	SpeechNoSpeech            = "no-speech"
	SpeechAudioCapture        = "audio-capture"
	SpeechNetwork             = "network"
	SpeechLanguageUnsupported = "language-not-supported"
	SpeechUnavailable         = "unavailable"
	SpeechFailed              = "failed"
)

// SpeechEvent is an EventSpeech's report.
type SpeechEvent struct {
	Phase SpeechPhase
	// Text is the whole transcript so far, and Final whether the
	// recognizer will not change it.
	Text  string
	Final bool
	// Error is one of the Speech errors, "" for none.
	Error string
}
