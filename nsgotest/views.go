package nsgotest

import "github.com/oneo-me/nsgo/internal/driver"

// The types of the view model, which mirrors what the platforms mount.

type (
	// AnimatedProp is a view property an animated node drives.
	AnimatedProp = driver.AnimatedProp
	// AnimatedValues are the bound properties of a view, as the animator last
	// evaluated them.
	AnimatedValues = driver.AnimatedValues
	// EventKind is what a platform reports about a view.
	EventKind = driver.EventKind
	// EventMask is a set of event kinds.
	EventMask = driver.EventMask
	// ImageConfig is an image view's source and look.
	ImageConfig = driver.ImageConfig
	// InputConfig is a text field's look and behavior.
	InputConfig = driver.InputConfig
	// ModalConfig is how a modal appears and goes.
	ModalConfig = driver.ModalConfig
	// ScreenConfig is a screen's system header, which the platform draws over
	// the top of the screen's view, HeaderHeight tall.
	ScreenConfig = driver.ScreenConfig
	// ScrollConfig configures a scroll view.
	ScrollConfig = driver.ScrollConfig
	// ScrollEdge is a scroll view's edge effect at one of its sides: the
	// effect, how far its bars reach and the bars.
	ScrollEdge = driver.ScrollEdge
	// StatusBarConfig is the status bar the platform shows (ItemStatusBar).
	StatusBarConfig = driver.StatusBarConfig
	// TextStyle is resolved text style: nothing in it is unset.
	TextStyle = driver.TextStyle
	// TextSpan styles a byte range of a text over its style.
	TextSpan = driver.TextSpan
	// TextBackground is what a span draws behind its characters.
	TextBackground = driver.TextBackground
	// VideoConfig is everything a video plays with (ItemVideo), with the
	// defaults applied: Volume in 0 to 1, Rate above 0, a ResizeMode and
	// ProgressInterval at least 16 milliseconds.
	VideoConfig = driver.VideoConfig
	// ViewKind is the kind of native view a node mounts.
	ViewKind = driver.ViewKind
	// ViewProps is a view's accessibility and identity.
	ViewProps = driver.ViewProps
	// ViewStyle is everything a platform draws for a view itself.
	ViewStyle = driver.ViewStyle
	// WebCommandKind is what ItemWebCommand asks a web view to do.
	WebCommandKind = driver.WebCommand
	// WebConfig is everything a web view loads and allows (ItemWeb).
	WebConfig = driver.WebConfig
)

const (
	PropOpacity       = driver.PropOpacity
	PropTranslateX    = driver.PropTranslateX
	PropTranslateY    = driver.PropTranslateY
	PropScale         = driver.PropScale
	PropScaleX        = driver.PropScaleX
	PropScaleY        = driver.PropScaleY
	PropRotate        = driver.PropRotate
	PropBackground    = driver.PropBackground
	PropBorderColor   = driver.PropBorderColor
	PropShadowColor   = driver.PropShadowColor
	PropShadowRadius  = driver.PropShadowRadius
	PropShadowOpacity = driver.PropShadowOpacity
	PropTextColor     = driver.PropTextColor
	PropRotateX       = driver.PropRotateX
	PropRotateY       = driver.PropRotateY
	PropCount         = driver.PropCount
)

const (
	EventPress                 = driver.EventPress
	EventPressDown             = driver.EventPressDown
	EventPressUp               = driver.EventPressUp
	EventLongPress             = driver.EventLongPress
	EventHoverIn               = driver.EventHoverIn
	EventHoverOut              = driver.EventHoverOut
	EventFocus                 = driver.EventFocus
	EventBlur                  = driver.EventBlur
	EventChangeText            = driver.EventChangeText
	EventSubmitEditing         = driver.EventSubmitEditing
	EventScroll                = driver.EventScroll
	EventLoadStart             = driver.EventLoadStart
	EventProgress              = driver.EventProgress
	EventLoad                  = driver.EventLoad
	EventError                 = driver.EventError
	EventLoadEnd               = driver.EventLoadEnd
	EventPressOutside          = driver.EventPressOutside
	EventBack                  = driver.EventBack
	EventAlert                 = driver.EventAlert
	EventClipboard             = driver.EventClipboard
	EventContextMenu           = driver.EventContextMenu
	EventKey                   = driver.EventKey
	EventPan                   = driver.EventPan
	EventPinch                 = driver.EventPinch
	EventRotation              = driver.EventRotation
	EventMenu                  = driver.EventMenu
	EventMessage               = driver.EventMessage
	EventNavigationStateChange = driver.EventNavigationStateChange
	EventEnd                   = driver.EventEnd
	EventBuffer                = driver.EventBuffer
	EventFileDragEnter         = driver.EventFileDragEnter
	EventFileDragLeave         = driver.EventFileDragLeave
	EventFileDrop              = driver.EventFileDrop
	EventNative                = driver.EventNative
	EventNativeSize            = driver.EventNativeSize
	EventAnimation             = driver.EventAnimation
	EventScrollDragStart       = driver.EventScrollDragStart
	EventScrollDragEnd         = driver.EventScrollDragEnd
	EventMomentumStart         = driver.EventMomentumStart
	EventMomentumEnd           = driver.EventMomentumEnd
	EventRefresh               = driver.EventRefresh
	EventImageInfo             = driver.EventImageInfo
	EventSelectionChange       = driver.EventSelectionChange
	EventKeyPress              = driver.EventKeyPress
	EventCompositionChange     = driver.EventCompositionChange
	EventAccessibilityAction   = driver.EventAccessibilityAction
	EventRequestClose          = driver.EventRequestClose
	EventModalSize             = driver.EventModalSize
	EventHoverMove             = driver.EventHoverMove
	EventMouseMove             = driver.EventMouseMove
	EventTouch                 = driver.EventTouch
	EventCaretMove             = driver.EventCaretMove
	EventPaste                 = driver.EventPaste
)

const (
	KindView   = driver.KindView
	KindText   = driver.KindText
	KindInput  = driver.KindInput
	KindScroll = driver.KindScroll
	KindImage  = driver.KindImage
	KindModal  = driver.KindModal
	KindCanvas = driver.KindCanvas
	KindWeb    = driver.KindWeb
	KindVideo  = driver.KindVideo
	KindNative = driver.KindNative
	KindShader = driver.KindShader
)

const (
	WebReload           = driver.WebReload
	WebGoBack           = driver.WebGoBack
	WebGoForward        = driver.WebGoForward
	WebStopLoading      = driver.WebStopLoading
	WebPostMessage      = driver.WebPostMessage
	WebInjectJavaScript = driver.WebInjectJavaScript
)
