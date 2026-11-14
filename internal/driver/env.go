package driver

import (
	"reflect"
	"slices"
	"strconv"
	"time"
)

// Environment is what the platform reports about where the app runs. The
// surface size is not part of it: Tree.Resize reports that.
type Environment struct {
	// DisplayFeatures are folds, hinges, cutouts and system bars in window
	// coordinates. Empty on targets without a foldable display.
	DisplayFeatures []DisplayFeature
	// ReservedRegions are areas occupied by a system feature and excluded
	// from app content, in addition to SafeArea.
	ReservedRegions []Bounds
	// SafeArea is the part of each edge that system bars, notches and
	// rounded corners cover.
	SafeArea Edges
	// TitleBar is what a hidden desktop title bar still covers: its
	// height on top, its buttons on the left or right.
	TitleBar Edges
	// WindowMaterial is the material behind the window's content; None
	// when the window is opaque.
	WindowMaterial WindowMaterial
	// Maximized is whether the window is maximized or zoomed, so an
	// app-drawn maximize button can show restore.
	Maximized bool
	// FullScreen is whether the window fills its screen in full screen.
	FullScreen bool
	// WindowFrame is where the window is and how large, as WindowSetFrame
	// takes them; zero without a window, and a position only where the
	// platform knows one (Move).
	WindowFrame WindowFrame
	// KeyboardHeight is how much of the bottom the software keyboard
	// covers; KeyboardDuration is how long it takes to move.
	KeyboardHeight   Points
	KeyboardDuration time.Duration
	Dark             bool
	// FontScale multiplies text sizes, as the user's text size setting
	// asks; 0 means 1.
	FontScale    float32
	ReduceMotion bool
	ScreenReader bool
	// ScreenColor says ItemPickScreenColor works here.
	ScreenColor bool
	// Speech says ItemSpeech recognizes speech here.
	Speech bool
	// Camera says ItemTakePhoto shows a camera here.
	Camera bool
	// RTL lays the app out right to left.
	RTL bool
	// PixelRatio is device pixels per point; 0 means 1.
	PixelRatio float32
	// Locale is a BCP 47 language tag, such as "en-US".
	Locale string
	// LocaleSettings are how the user reads weeks, times and measures,
	// apart from the language, where the platform says.
	LocaleSettings LocaleSettings
	OS             OS
	// OSVersion is the system's version: major and minor on Apple
	// platforms, Windows and Linux (the kernel's), Android's API level
	// as Major, and zero on the web.
	OSVersion OSVersion
	Idiom     Idiom
	// AppleKeyboard says a browser runs on an Apple platform, where "mod"
	// is Command as on macOS and iOS.
	AppleKeyboard bool
	// HeaderHeight and LargeHeaderHeight are the heights of the system
	// header a platform draws over screens, without the status bar, and
	// with a large title. 0 where screens show their Header instead.
	HeaderHeight, LargeHeaderHeight Points
	// Lifecycle is whether the app is in use: the window's, an embedded
	// surface's the host window's.
	Lifecycle Lifecycle
	// AppClip says the app runs as an iOS App Clip, and OpenedURL is the
	// web link that last opened it: the clip's invocation, or a universal
	// link; "" when none did.
	AppClip   bool
	OpenedURL string
}

// Equal reports whether two environments carry the same platform state.
// Slices are compared by value so Environment remains convenient to copy.
func (e Environment) Equal(other Environment) bool {
	a, b := e, other
	features, regions := a.DisplayFeatures, a.ReservedRegions
	otherFeatures, otherRegions := b.DisplayFeatures, b.ReservedRegions
	a.DisplayFeatures, a.ReservedRegions = nil, nil
	b.DisplayFeatures, b.ReservedRegions = nil, nil
	return reflect.DeepEqual(a, b) && slices.Equal(features, otherFeatures) && slices.Equal(regions, otherRegions)
}

// DisplayFeatureKind describes a physical or system feature of a display.
type DisplayFeatureKind uint8

const (
	DisplayFeatureFold DisplayFeatureKind = iota
	DisplayFeatureHinge
	DisplayFeatureCutout
	DisplayFeatureBar
)

// DisplayFeatureState is the posture of a fold or hinge.
type DisplayFeatureState uint8

const (
	DisplayFeatureFlat DisplayFeatureState = iota
	DisplayFeatureHalfFolded
	DisplayFeatureFolded
)

// DisplayFeature is a display feature in window coordinates.
type DisplayFeature struct {
	Kind   DisplayFeatureKind
	State  DisplayFeatureState
	Bounds Bounds
}

// LocaleSettings are the regional preferences the user set, in Intl's
// names: FirstDayOfWeek 1 for Monday to 7 for Sunday (ISO 8601),
// HourCycle "h11", "h12", "h23" or "h24", MeasurementSystem "metric",
// "us" or "uk"; zero where the platform does not say, which Region
// decides, else the locale's region (i18n.Regional). Region is the region
// whose formats the user chose apart from the language, such as Linux's
// LC_TIME, an ISO 3166 code.
type LocaleSettings struct {
	FirstDayOfWeek    int
	HourCycle         string
	MeasurementSystem string
	Region            string
}

// Lifecycle is whether the app is in use, as NSH's lifecycle() reads it.
type Lifecycle uint8

const (
	// LifecycleActive is an app that shows and has the focus: on desktops
	// one of its windows is the active one.
	LifecycleActive Lifecycle = iota
	// LifecycleInactive is an app that shows without the focus, as behind
	// another app's window, an incoming call or the app switcher.
	LifecycleInactive
	// LifecycleBackground is an app none of which shows: hidden,
	// minimized, covered where the system reports it, or in the
	// background on phones.
	LifecycleBackground
)

// String returns the state's name in TSX: active, inactive or background.
func (l Lifecycle) String() string {
	switch l {
	case LifecycleInactive:
		return "inactive"
	case LifecycleBackground:
		return "background"
	}
	return "active"
}

// OS names the operating system the app runs on.
type OS string

const (
	OSMacOS   OS = "macos"
	OSIOS     OS = "ios"
	OSAndroid OS = "android"
	OSWindows OS = "windows"
	OSLinux   OS = "linux"
	OSWeb     OS = "web"
)

// Idiom is the kind of device, which decides how components present.
type Idiom uint8

const (
	IdiomPhone Idiom = iota
	IdiomTablet
	// IdiomDesktop is a desktop window, or a browser with a mouse and a
	// wide page.
	IdiomDesktop
	// IdiomMac is an iPhone or iPad app running on a Mac.
	IdiomMac
	// IdiomTV is a television, such as Android TV.
	IdiomTV
	// IdiomWatch is a watch: Apple Watch.
	IdiomWatch
)

// OSVersion is an operating system's version.
type OSVersion struct {
	Major, Minor int
}

// AtLeast reports whether v is major.minor or later.
func (v OSVersion) AtLeast(major, minor int) bool {
	return v.Major > major || v.Major == major && v.Minor >= minor
}

// String returns v as major.minor.
func (v OSVersion) String() string { return strconv.Itoa(v.Major) + "." + strconv.Itoa(v.Minor) }
