//go:build darwin

package apple

import (
	"runtime"
	"strings"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

var (
	dispatchAsyncF   func(queue, context, work uintptr)
	poolPush         func() uintptr
	poolPop          func(uintptr)
	mainQueue        uintptr
	mainWork         uintptr
	msgSend          uintptr
	setQOS           func(class uint32, priority int32) int32
	mainMu           sync.Mutex
	mainFuncs        []func()
	selAddObserver   = sel("addObserverForName:object:queue:usingBlock:")
	selMainQueue     = sel("mainQueue")
	selCount         = sel("count")
	selObjectAtIndex = sel("objectAtIndex:")
	selObjectForKey  = sel("objectForKey:")
)

func loadSystem() {
	lib := load("/usr/lib/libSystem.B.dylib")
	purego.RegisterLibFunc(&dispatchAsyncF, lib, "dispatch_async_f")
	mainQueue = symbol(lib, "_dispatch_main_q")
	mainWork = purego.NewCallback(func(uintptr) { runMain() })
	runtimeLib := load("/usr/lib/libobjc.A.dylib")
	purego.RegisterLibFunc(&poolPush, runtimeLib, "objc_autoreleasePoolPush")
	purego.RegisterLibFunc(&poolPop, runtimeLib, "objc_autoreleasePoolPop")
	msgSend = symbol(runtimeLib, "objc_msgSend")
	purego.RegisterLibFunc(&setQOS, lib, "pthread_set_qos_class_self_np")
}

// qosUserInteractive is QOS_CLASS_USER_INTERACTIVE, the main thread's.
const qosUserInteractive = 0x21

// RaiseThread gives the calling OS thread the main thread's quality of
// service, as NSH gives its engine thread. The engine's thread turns input
// into frames; at the default class the system may run it on efficiency
// cores, and the quotes benchmark's commits took about 15% longer and
// varied with the machine's load.
func RaiseThread() { setQOS(qosUserInteractive, 0) }

// Msg sends sel to id with up to six arguments and a result that are
// objects, selectors, pointers, integers or booleans, through
// purego.SyscallN. objc.Send and ID.Send work each call out with
// reflection, which costs several times the message itself; what runs for
// every changed view of every frame sends with Msg.
func Msg(id objc.ID, sel objc.SEL, args ...uintptr) uintptr {
	var all [8]uintptr
	if len(args) > len(all)-2 {
		panic("apple: too many arguments to Msg")
	}
	all[0], all[1] = uintptr(id), uintptr(sel)
	n := 2 + copy(all[2:], args)
	r, _, _ := purego.SyscallN(msgSend, all[:n]...)
	return r
}

// MsgBool is Msg for a BOOL result.
func MsgBool(id objc.ID, sel objc.SEL, args ...uintptr) bool {
	return byte(Msg(id, sel, args...)) != 0
}

// OnMain runs fn on the main thread after what is queued there already.
// It never waits, so any goroutine may call it.
func OnMain(fn func()) {
	mainMu.Lock()
	mainFuncs = append(mainFuncs, fn)
	first := len(mainFuncs) == 1
	mainMu.Unlock()
	if first {
		dispatchAsyncF(mainQueue, 0, mainWork)
	}
}

func runMain() {
	mainMu.Lock()
	fns := mainFuncs
	mainFuncs = nil
	mainMu.Unlock()
	for _, fn := range fns {
		fn()
	}
}

// withPool runs fn on one OS thread inside an autorelease pool, for
// Objective-C calls off the main thread.
func withPool(fn func()) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pool := poolPush()
	defer poolPop(pool)
	fn()
}

// WithoutAnimation runs fn with Core Animation's implicit animations off,
// so layers a backend adds itself change at once.
func WithoutAnimation(fn func()) {
	tx := objc.ID(Class("CATransaction"))
	tx.Send(sel("begin"))
	tx.Send(sel("setDisableActions:"), true)
	fn()
	tx.Send(sel("commit"))
}

// Observe calls fn on the main thread each time center posts name, until
// stop is called.
func Observe(center, name objc.ID, fn func(note objc.ID)) (stop func()) {
	if center == 0 || name == 0 {
		return func() {}
	}
	block := objc.NewBlock(func(_ objc.Block, note objc.ID) { fn(note) })
	queue := objc.ID(Class("NSOperationQueue")).Send(selMainQueue)
	token := center.Send(selAddObserver, name, objc.ID(0), queue, block).Send(sel("retain"))
	block.Release()
	return func() {
		if token != 0 {
			center.Send(sel("removeObserver:"), token)
			token.Send(sel("release"))
			token = 0
		}
	}
}

// Global reads the global object, such as an NSString constant, that
// framework at path exports as name; 0 when it has none.
func Global(path, name string) objc.ID {
	lib, err := purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return 0
	}
	addr, err := purego.Dlsym(lib, name)
	if err != nil || addr == 0 {
		return 0
	}
	return **(**objc.ID)(unsafe.Pointer(&addr))
}

// CommonModes returns NSRunLoopCommonModes. Run loop sources compare
// modes by identity, so an equal string of another object is a mode of
// its own that never runs.
func CommonModes() objc.ID {
	commonModesOnce.Do(func() {
		commonModes = Global("/System/Library/Frameworks/Foundation.framework/Foundation", "NSRunLoopCommonModes")
	})
	return commonModes
}

var (
	commonModesOnce sync.Once
	commonModes     objc.ID
)

// Locale returns the user's locale as a BCP 47 tag, and whether their
// first preferred language is written right to left.
func Locale() (tag string, rtl bool) {
	nsLocale := objc.ID(Class("NSLocale"))
	id := GoString(nsLocale.Send(sel("currentLocale")).Send(sel("localeIdentifier")))
	if i := strings.IndexByte(id, '@'); i >= 0 {
		id = id[:i]
	}
	tag = strings.ReplaceAll(id, "_", "-")
	langs := nsLocale.Send(sel("preferredLanguages"))
	if objc.Send[uint](langs, selCount) > 0 {
		const rightToLeft = 2
		first := langs.Send(selObjectAtIndex, uint(0))
		rtl = objc.Send[int](nsLocale, sel("characterDirectionForLanguage:"), first) == rightToLeft
	}
	return tag, rtl
}

// LocaleSettings returns the regional preferences the user set in
// Language & Region: the first day of the week (NSCalendar's), the hours
// of NSDateFormatter's format for the j template, which 24-hour time
// changes, and NSLocale's measurement system.
func LocaleSettings() driver.LocaleSettings {
	var s driver.LocaleSettings
	locale := objc.ID(Class("NSLocale")).Send(sel("currentLocale"))
	// NSCalendar numbers Sunday 1.
	if first := objc.Send[int](objc.ID(Class("NSCalendar")).Send(sel("currentCalendar")), sel("firstWeekday")); first >= 1 && first <= 7 {
		s.FirstDayOfWeek = (first+5)%7 + 1
	}
	format := GoString(objc.ID(Class("NSDateFormatter")).Send(sel("dateFormatFromTemplate:options:locale:"), NSString("j"), uint(0), locale))
	for _, r := range format {
		if c := map[rune]string{'H': "h23", 'h': "h12", 'K': "h11", 'k': "h24"}[r]; c != "" {
			s.HourCycle = c
			break
		}
	}
	key := Global("/System/Library/Frameworks/Foundation.framework/Foundation", "NSLocaleMeasurementSystem")
	if key != 0 {
		switch GoString(locale.Send(sel("objectForKey:"), key)) {
		case "Metric":
			s.MeasurementSystem = "metric"
		case "U.S.":
			s.MeasurementSystem = "us"
		case "U.K.":
			s.MeasurementSystem = "uk"
		}
	}
	return s
}

// ForEach calls fn with each object of an NSArray or NSSet.
func ForEach(collection objc.ID, fn func(objc.ID)) {
	if objc.Send[bool](collection, sel("isKindOfClass:"), Class("NSSet")) {
		collection = collection.Send(sel("allObjects"))
	}
	n := objc.Send[uint](collection, selCount)
	for i := range n {
		fn(collection.Send(selObjectAtIndex, i))
	}
}

// KeyNames returns the names core.NormalizeKey gives a key press: the key
// named name with the modifier flags AppKit and UIKit share, first with
// Command as "mod", then as "meta".
func KeyNames(name string, flags uint) []string {
	const shift, control, option, command = 1 << 17, 1 << 18, 1 << 19, 1 << 20
	var mods []string
	for _, m := range []struct {
		flag uint
		name string
	}{{control, "ctrl"}, {option, "alt"}, {shift, "shift"}} {
		if flags&m.flag != 0 {
			mods = append(mods, m.name)
		}
	}
	if flags&command == 0 {
		return []string{strings.Join(append(mods, name), "+")}
	}
	return []string{
		strings.Join(append(append([]string{"mod"}, mods...), name), "+"),
		strings.Join(append(append([]string{}, mods...), "meta", name), "+"),
	}
}

// The defaults keys of a language the app forces: AppleLanguages, which
// the system reads as the app launches, as System Settings' language of
// an app sets it, and NSGoLanguage, what NSGo last wrote there.
const (
	appleLanguagesKey = "AppleLanguages"
	pinnedLanguageKey = "NSGoLanguage"
)

// PinLanguage keeps lang, the language the app forces, for what the
// system says in the app from its next launch: the bundle's name in the
// menu bar, the items it adds to menus, its panels and its controls'
// words, which follow the languages of the launch and not the app's.
// "" lets them follow the user's languages again, unless the user chose
// the app's language in System Settings since. A pseudo locale has no
// localization of the system's, and pins nothing; neither does a process
// outside a bundle.
func PinLanguage(lang string) {
	if lang == "en-XA" || lang == "ar-XB" { // i18n's pseudo locales
		lang = ""
	}
	id := objc.ID(Class("NSBundle")).Send(sel("mainBundle")).Send(sel("bundleIdentifier"))
	if id == 0 {
		return
	}
	defaults := objc.ID(Class("NSUserDefaults")).Send(sel("standardUserDefaults"))
	var pinned, chosen string
	if domain := defaults.Send(sel("persistentDomainForName:"), id); domain != 0 {
		pinned = GoString(domain.Send(selObjectForKey, NSString(pinnedLanguageKey)))
		if langs := domain.Send(selObjectForKey, NSString(appleLanguagesKey)); langs != 0 && objc.Send[uint](langs, selCount) > 0 {
			chosen = GoString(langs.Send(selObjectAtIndex, uint(0)))
		}
	}
	switch {
	case lang != "" && (lang != pinned || lang != chosen):
		langs := objc.ID(Class("NSArray")).Send(sel("arrayWithObject:"), NSString(lang))
		defaults.Send(sel("setObject:forKey:"), langs, NSString(appleLanguagesKey))
		defaults.Send(sel("setObject:forKey:"), NSString(lang), NSString(pinnedLanguageKey))
	case lang == "" && pinned != "":
		if chosen == pinned {
			defaults.Send(sel("removeObjectForKey:"), NSString(appleLanguagesKey))
		}
		defaults.Send(sel("removeObjectForKey:"), NSString(pinnedLanguageKey))
	}
}

// BundleName returns the app's name in lang as its bundle's
// InfoPlist.strings says it, in the localization that suits lang best;
// "" when the bundle names it in none.
func BundleName(lang string) string {
	if lang == "" {
		return ""
	}
	bundle := objc.ID(Class("NSBundle")).Send(sel("mainBundle"))
	prefs := objc.ID(Class("NSArray")).Send(sel("arrayWithObject:"), NSString(lang))
	locs := objc.ID(Class("NSBundle")).Send(sel("preferredLocalizationsFromArray:forPreferences:"),
		bundle.Send(sel("localizations")), prefs)
	if locs == 0 || objc.Send[uint](locs, selCount) == 0 {
		return ""
	}
	path := bundle.Send(sel("pathForResource:ofType:inDirectory:forLocalization:"), NSString("InfoPlist"), NSString("strings"),
		objc.ID(0), locs.Send(selObjectAtIndex, uint(0)))
	if path == 0 {
		return ""
	}
	strs := objc.ID(Class("NSDictionary")).Send(sel("dictionaryWithContentsOfFile:"), path)
	if strs == 0 {
		return ""
	}
	for _, key := range []string{"CFBundleDisplayName", "CFBundleName"} {
		if name := GoString(strs.Send(selObjectForKey, NSString(key))); name != "" {
			return name
		}
	}
	return ""
}
