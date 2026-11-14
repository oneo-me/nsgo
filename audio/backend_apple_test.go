//go:build darwin && !ios

package audio

import (
	"encoding/binary"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ebitengine/purego/objc"
)

// TestMain runs the main thread's run loop, as an app does: AVAudioPlayer
// reports to its delegate there.
func TestMain(m *testing.M) {
	go func() { os.Exit(m.Run()) }()
	loop := objc.ID(objc.GetClass("NSRunLoop")).Send(objc.RegisterName("currentRunLoop"))
	// A run loop with no port to wait on returns at once.
	port := objc.ID(objc.GetClass("NSMachPort")).Send(objc.RegisterName("port"))
	loop.Send(objc.RegisterName("addPort:forMode:"), port, objc.ID(objc.GetClass("NSString")).Send(objc.RegisterName("stringWithUTF8String:"), "kCFRunLoopDefaultMode"))
	loop.Send(objc.RegisterName("run"))
}

// tone returns a WAV of length's silence: 16-bit mono at 8 kHz.
func tone(length time.Duration) []byte {
	const rate = 8000
	samples := int(length.Seconds() * rate)
	wav := make([]byte, 44+2*samples)
	copy(wav, "RIFF")
	binary.LittleEndian.PutUint32(wav[4:], uint32(36+2*samples))
	copy(wav[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(wav[16:], 16)
	binary.LittleEndian.PutUint16(wav[20:], 1) // PCM
	binary.LittleEndian.PutUint16(wav[22:], 1) // mono
	binary.LittleEndian.PutUint32(wav[24:], rate)
	binary.LittleEndian.PutUint32(wav[28:], 2*rate)
	binary.LittleEndian.PutUint16(wav[32:], 2)
	binary.LittleEndian.PutUint16(wav[34:], 16)
	copy(wav[36:], "data")
	binary.LittleEndian.PutUint32(wav[40:], uint32(2*samples))
	return wav
}

// needOutput skips a test on a Mac that has no output device to play on.
func needOutput(t *testing.T, err error) {
	t.Helper()
	if errors.Is(err, ErrDeviceUnavailable) {
		t.Skip("no audio output device")
	}
	if err != nil {
		t.Fatal(err)
	}
}

func waitFor(t *testing.T, what string, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatalf("%s did not happen", what)
	}
}

func TestAppleCueEnds(t *testing.T) {
	sound, err := PlayBytes(tone(200*time.Millisecond), "audio/wav", WithVolume(0))
	needOutput(t, err)
	defer sound.Close()
	ended := make(chan struct{})
	if err := sound.OnEnd(func() { close(ended) }); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the cue's end", ended)
}

func TestApplePlayerEnds(t *testing.T) {
	player, err := OpenBytes(tone(300*time.Millisecond), "audio/wav", WithVolume(0), WithProgressInterval(50*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	defer player.Close()
	ended := make(chan struct{})
	if err := player.OnEnd(func() { close(ended) }); err != nil {
		t.Fatal(err)
	}
	needOutput(t, player.Play())
	waitFor(t, "the player's end", ended)
	if !player.Ended() || player.Playing() {
		t.Errorf("ended=%v playing=%v, want true/false", player.Ended(), player.Playing())
	}
	if got, want := player.Position(), player.Duration(); got != want || want == 0 {
		t.Errorf("position %s after the end, want the duration %s", got, want)
	}
	// No progress follows the end.
	late := make(chan struct{}, 1)
	player.OnProgress(func(time.Duration, time.Duration) { late <- struct{}{} })
	select {
	case <-late:
		t.Error("progress was reported after the end")
	case <-time.After(200 * time.Millisecond):
	}
}

func TestApplePlayerLoopsAndSeeksBack(t *testing.T) {
	player, err := OpenBytes(tone(300*time.Millisecond), "audio/wav", WithVolume(0), WithLoop(true), WithProgressInterval(50*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	defer player.Close()
	needOutput(t, player.Play())
	// Over three loops the position must come back towards the start.
	var highest time.Duration
	wrapped := false
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline) && !wrapped; time.Sleep(20 * time.Millisecond) {
		position := player.Position()
		wrapped = position < highest-100*time.Millisecond
		highest = max(highest, position)
	}
	if !wrapped {
		t.Errorf("the position never went back as the player looped; highest %s", highest)
	}
	if err := player.Pause(); err != nil {
		t.Fatal(err)
	}
	if err := player.Seek(50 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if got := player.Position(); got < 40*time.Millisecond || got > 60*time.Millisecond {
		t.Errorf("position %s after seeking back to 50ms", got)
	}
}

func TestAppleRejectsPathsItCannotName(t *testing.T) {
	if _, err := Open("/tmp/\xff.wav"); !errors.Is(err, ErrUnreadableSource) {
		t.Errorf("invalid UTF-8 path error %v, want ErrUnreadableSource", err)
	}
	if _, err := PlayFile("/tmp/\xff.wav"); !errors.Is(err, ErrUnreadableSource) {
		t.Errorf("invalid UTF-8 cue path error %v, want ErrUnreadableSource", err)
	}
}

func TestAppleTellsUnreadableFromUnsupported(t *testing.T) {
	dir := t.TempDir()
	if _, err := Open(filepath.Join(dir, "missing.wav")); !errors.Is(err, ErrUnreadableSource) || !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("missing file error %v, want ErrUnreadableSource and fs.ErrNotExist", err)
	}
	if _, err := Open(dir); !errors.Is(err, ErrUnreadableSource) {
		t.Errorf("directory error %v, want ErrUnreadableSource", err)
	}
	text := filepath.Join(dir, "text.wav")
	if err := os.WriteFile(text, []byte("not audio at all"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(text); !errors.Is(err, ErrUnsupportedFormat) {
		t.Errorf("text file error %v, want ErrUnsupportedFormat", err)
	}
	if _, err := OpenBytes([]byte("not audio at all"), "audio/wav"); !errors.Is(err, ErrUnsupportedFormat) {
		t.Errorf("text bytes error %v, want ErrUnsupportedFormat", err)
	}
	wav := filepath.Join(dir, "tone.wav")
	if err := os.WriteFile(wav, tone(100*time.Millisecond), 0o644); err != nil {
		t.Fatal(err)
	}
	player, err := Open(wav)
	if err != nil {
		t.Fatal(err)
	}
	player.Close()
	// A MIME type is a hint, not a requirement, and may carry parameters.
	for _, mime := range []string{"audio/wav; codecs=1", "audio/mpeg", "application/x-unknown"} {
		player, err := OpenBytes(tone(100*time.Millisecond), mime)
		if err != nil {
			t.Errorf("WAV bytes as %s: %v", mime, err)
			continue
		}
		player.Close()
	}
}
