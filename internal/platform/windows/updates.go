//go:build windows

package windows

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/updates"
)

// winSparkle is WinSparkle.dll from beside the executable, which nsgo
// build puts there when nsgo.yaml configures updates, once
// startWinSparkle started it. WinSparkle reads the app's appcast on its
// own schedule and shows its windows; it downloads a newer installer,
// checks its EdDSA signature, has the app quit and runs it, and the
// installer starts the new version.
var winSparkle *syscall.DLL

// updateWindow is the window WinSparkle's shutdown request closes, as
// Quit does.
var updateWindow uintptr

// WinSparkle's callbacks, made once: it may call them from any thread.
var (
	canShutdown     = syscall.NewCallbackCDecl(func() uintptr { return 1 })
	shutdownRequest = syscall.NewCallbackCDecl(func() uintptr {
		postMessageW.Call(updateWindow, wmClose, 0, 0)
		return 0
	})
)

// startWinSparkle starts WinSparkle on c's appcast, for the app's main
// window hwnd.
func startWinSparkle(c updates.Config, hwnd uintptr) {
	if c.Feed == "" {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	dll, err := syscall.LoadDLL(filepath.Join(filepath.Dir(exe), "WinSparkle.dll"))
	if err != nil {
		log.Printf("nsgo: nsgo.yaml configures updates but WinSparkle.dll does not load: %v", err)
		return
	}
	var keep []any
	cstr := func(s string) uintptr {
		p, _ := syscall.BytePtrFromString(s)
		keep = append(keep, p)
		return uintptr(unsafe.Pointer(p))
	}
	wstr := func(s string) uintptr {
		p := wide(s)
		keep = append(keep, p)
		return uintptr(unsafe.Pointer(p))
	}
	call := func(name string, args ...uintptr) uintptr {
		p, err := dll.FindProc(name)
		if err != nil {
			log.Printf("nsgo: WinSparkle.dll: %v", err)
			return 0
		}
		r, _, _ := p.Call(args...)
		return r
	}
	registry := `Software\` + c.ID + `\WinSparkle`
	call("win_sparkle_set_appcast_url", cstr(c.Feed))
	if call("win_sparkle_set_eddsa_public_key", cstr(c.Key)) == 0 {
		log.Printf("nsgo: WinSparkle refuses updates.key's public key: it installs no update")
	}
	call("win_sparkle_set_app_details", wstr(c.App), wstr(c.App), wstr(c.Version))
	if c.Build != "" && c.Build != c.Version {
		call("win_sparkle_set_app_build_version", wstr(c.Build))
	}
	call("win_sparkle_set_registry_path", cstr(registry))
	// nsgo.yaml's choice is the default; the user's, in WinSparkle's
	// settings, stays.
	if !registryValue(registry, "CheckForUpdates") {
		automatic := uintptr(0)
		if c.Automatic {
			automatic = 1
		}
		call("win_sparkle_set_automatic_check_for_updates", automatic)
	}
	call("win_sparkle_set_can_shutdown_callback", canShutdown)
	call("win_sparkle_set_shutdown_request_callback", shutdownRequest)
	call("win_sparkle_init")
	runtime.KeepAlive(keep)
	winSparkle, updateWindow = dll, hwnd
}

// stopWinSparkle stops WinSparkle's checks and downloads as the app ends.
func stopWinSparkle() {
	if winSparkle == nil {
		return
	}
	if p, err := winSparkle.FindProc("win_sparkle_cleanup"); err == nil {
		p.Call()
	}
	winSparkle = nil
}

// registryValue reports whether the current user's key path holds a value
// named name.
func registryValue(path, name string) bool {
	var key syscall.Handle
	if syscall.RegOpenKeyEx(syscall.HKEY_CURRENT_USER, wide(path), 0, syscall.KEY_READ, &key) != nil {
		return false
	}
	defer syscall.RegCloseKey(key)
	return syscall.RegQueryValueEx(key, wide(name), nil, nil, nil, nil) == nil
}

// checkForUpdates has WinSparkle check now and show what it finds, or in
// the background only a newer version.
func (w *window) checkForUpdates(background bool, id uint32) {
	ev := driver.Event{Kind: driver.EventUpdate, Count: id, Finished: winSparkle != nil}
	name := "win_sparkle_check_update_with_ui"
	if background {
		name = "win_sparkle_check_update_without_ui"
	}
	if winSparkle == nil {
		ev.Data = &driver.Update{Err: "updates: this build has no WinSparkle: nsgo.yaml configures no updates"}
	} else if p, err := winSparkle.FindProc(name); err == nil {
		p.Call()
	}
	w.engine.Dispatch(ev)
}
