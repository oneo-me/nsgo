//go:build windows

package windows

// The Direct3D 11, DXGI, Direct2D 1.1 and DirectWrite interfaces this
// backend calls, as vtable indexes (from d3d11.h, dxgi1_2.h, d2d1_1.h,
// d2d1effects.h and dwrite.h; each interface's methods follow its base's),
// and the structs they take.

import "syscall"

var (
	d3d11  = syscall.NewLazyDLL("d3d11.dll")
	d2d1   = syscall.NewLazyDLL("d2d1.dll")
	dwrite = syscall.NewLazyDLL("dwrite.dll")

	d3d11CreateDevice   = d3d11.NewProc("D3D11CreateDevice")
	d2d1CreateFactory   = d2d1.NewProc("D2D1CreateFactory")
	dwriteCreateFactory = dwrite.NewProc("DWriteCreateFactory")
)

var (
	iidDXGIDevice        = parseGUID("54ec77fa-1377-44e6-8c32-88fd5f44c84c")
	iidDXGIDevice1       = parseGUID("77db970f-6276-48ba-ba28-070143b4392c")
	iidDXGIFactory2      = parseGUID("50c83a1c-e072-4c48-87b0-3630fa36a6d0")
	iidDXGISurface       = parseGUID("cafcb56c-6ac3-4889-bf47-9e23bbd260ec")
	iidD3D11Multithread  = parseGUID("9b7e4e00-342c-4106-a19f-4f2704f689f0")
	iidD2D1Factory1      = parseGUID("bb12d362-daee-4b9a-aa1d-14ba401cfa1f")
	iidDWriteFactory     = parseGUID("b859ee5a-d838-4b5b-a2e8-1adc7d93db48")
	iidDWriteTextLayout1 = parseGUID("9064d822-80a7-465c-a986-df65f78b8feb")
	iidDWriteInline      = parseGUID("8339fde3-106f-47ab-8373-1c6295eb10b3")
	clsidGaussianBlur    = parseGUID("1feb6d69-2fe6-4ac9-8c58-1d7f93e7a6a5")
	clsidShadow          = parseGUID("c67ea361-1863-4e69-89db-695d3e9a5b6b")
)

// Vtable indexes.
const (
	// ID3D11Multithread
	multithreadSetProtected = 5

	// IDXGIObject, IDXGIDevice, IDXGIDevice1
	dxgiGetParent                = 6
	dxgiDeviceGetAdapter         = 7
	dxgiDeviceSetMaxFrameLatency = 12

	// IDXGIFactory, IDXGIFactory2
	dxgiFactoryMakeWindowAssociation  = 8
	dxgiFactoryCreateSwapChainForHwnd = 15

	// IDXGISwapChain
	swapPresent       = 8
	swapGetBuffer     = 9
	swapResizeBuffers = 13
	swapPresent1      = 22

	// ID2D1Factory, ID2D1Factory1
	d2dFactoryCreateRoundedRectangleGeometry = 6
	d2dFactoryCreatePathGeometry             = 10
	d2dFactoryCreateStrokeStyle              = 11
	d2dFactoryCreateDevice                   = 17

	// ID2D1Device
	d2dDeviceCreateDeviceContext = 4

	// ID2D1RenderTarget
	rtCreateSolidColorBrush = 8
	rtDrawRectangle         = 16
	rtFillRectangle         = 17
	rtDrawRoundedRectangle  = 18
	rtFillRoundedRectangle  = 19
	rtDrawGeometry          = 22
	rtFillEllipse           = 21
	rtFillGeometry          = 23
	rtDrawBitmap            = 26
	rtDrawText              = 27
	rtDrawTextLayout        = 28
	rtSetTransform          = 30
	rtPopLayer              = 41
	rtPushAxisAlignedClip   = 45
	rtPopAxisAlignedClip    = 46
	rtClear                 = 47
	rtBeginDraw             = 48
	rtEndDraw               = 49
	rtSetDpi                = 51

	// ID2D1DeviceContext
	dcCreateBitmap                = 57
	dcCreateBitmapFromDxgiSurface = 62
	dcCreateEffect                = 63
	dcCreateCommandList           = 67
	dcSetTarget                   = 74
	dcDrawImage                   = 83
	dcPushLayer                   = 86

	// ID2D1SolidColorBrush
	brushSetColor = 8

	// ID2D1PathGeometry, ID2D1GeometrySink
	pathOpen        = 17
	sinkSetFillMode = 3
	sinkBeginFigure = 5
	sinkAddLines    = 6
	sinkAddBeziers  = 7
	sinkEndFigure   = 8
	sinkClose       = 9
	sinkAddArc      = 14

	// ID2D1CommandList
	commandListClose = 5

	// ID2D1Properties, ID2D1Effect
	effectSetValue  = 9
	effectSetInput  = 14
	effectGetOutput = 18

	// IDWriteFactory
	dwGetSystemFontCollection    = 3
	dwCreateTextFormat           = 15
	dwCreateTypography           = 16
	dwCreateTextLayout           = 18
	dwCreateEllipsisTrimmingSign = 20

	// IDWriteTypography
	typographyAddFontFeature = 3

	// IDWriteFontCollection, IDWriteFontFamily, IDWriteFont
	collectionGetFontFamily    = 4
	collectionFindFamilyName   = 5
	familyGetFirstMatchingFont = 7
	fontGetMetrics             = 11

	// IDWriteTextFormat
	formatSetTextAlignment      = 3
	formatSetParagraphAlignment = 4
	formatSetWordWrapping       = 5
	formatSetReadingDirection   = 6
	formatSetTrimming           = 9
	formatSetLineSpacing        = 10

	// IDWriteTextLayout, IDWriteTextLayout1
	layoutSetMaxWidth         = 28
	layoutSetMaxHeight        = 29
	layoutSetFontFamilyName   = 31
	layoutSetFontWeight       = 32
	layoutSetFontStyle        = 33
	layoutSetFontSize         = 35
	layoutSetDrawingEffect    = 38
	layoutSetInlineObject     = 39
	layoutSetTypography       = 40
	layoutGetLineMetrics      = 59
	layoutGetMetrics          = 60
	layoutHitTestPoint        = 64
	layoutHitTestTextPosition = 65
	layoutHitTestTextRange    = 66
	layoutSetCharacterSpacing = 69
)

// Enumerations and flags.
const (
	d3dDriverHardware          = 1
	d3dDriverWARP              = 5
	d3dBGRASupport             = 0x20
	d3dVideoSupport            = 0x800
	d3dSDKVersion              = 7
	dxgiFormatBGRA             = 87 // DXGI_FORMAT_B8G8R8A8_UNORM
	dxgiUsageTarget            = 0x20
	dxgiFlipSequential         = 3
	dxgiFlipDiscard            = 4
	dxgiAlphaIgnore            = 3
	dxgiMWANoAltEnter          = 2
	d2dAlphaPremultiplied      = 1
	d2dAlphaIgnore             = 3
	d2dBitmapTarget            = 1
	d2dBitmapCannotDraw        = 2
	d2dAntialiasPerPrimitive   = 0
	d2dAntialiasAliased        = 1
	d2dInterpolationLinear     = 1
	d2dFigureFilled            = 0
	d2dFigureHollow            = 1
	d2dFigureOpen              = 0
	d2dFigureClosed            = 1
	d2dFillAlternate           = 0
	d2dFillWinding             = 1
	d2dSweepClockwise          = 1
	d2dPropertyFloat           = 5
	d2dPropertyVector4         = 8
	d2dDrawTextEnableColorFont = 4
	d2dCapFlat                 = 0
	d2dCapSquare               = 1
	d2dCapRound                = 2
	d2dJoinMiter               = 0
	d2dJoinBevel               = 1
	d2dJoinRound               = 2
	d2dJoinMiterOrBevel        = 3

	d2dErrRecreateTarget = 0x8899000C
	dxgiErrDeviceRemoved = 0x887A0005
	dxgiErrDeviceReset   = 0x887A0007

	dwFactoryShared      = 0
	dwStretchNormal      = 5
	dwStyleNormal        = 0
	dwStyleItalic        = 2
	dwAlignLeading       = 0
	dwAlignTrailing      = 1
	dwAlignCenter        = 2
	dwAlignJustified     = 3
	dwParagraphCenter    = 2
	dwWrap               = 0
	dwNoWrap             = 1
	dwReadingRTL         = 1 // DWRITE_READING_DIRECTION_RIGHT_TO_LEFT
	dwLineSpacingUniform = 1
	dwTrimmingNone       = 0
	dwTrimmingCharacter  = 1
)

// rectF is D2D1_RECT_F.
type rectF struct{ left, top, right, bottom float32 }

func (r rectF) empty() bool { return r.right <= r.left || r.bottom <= r.top }

func (r rectF) intersect(o rectF) rectF {
	return rectF{max(r.left, o.left), max(r.top, o.top), min(r.right, o.right), min(r.bottom, o.bottom)}
}

// pointF is D2D1_POINT_2F.
type pointF struct{ x, y float32 }

// colorF is D2D1_COLOR_F: straight alpha, which Direct2D premultiplies.
type colorF struct{ r, g, b, a float32 }

// matrix3x2 is D2D1_MATRIX_3X2_F.
type matrix3x2 struct{ m11, m12, m21, m22, dx, dy float32 }

// ellipse is D2D1_ELLIPSE.
type ellipse struct {
	center           pointF
	radiusX, radiusY float32
}

// roundedRect is D2D1_ROUNDED_RECT.
type roundedRect struct {
	rect             rectF
	radiusX, radiusY float32
}

// arcSegment is D2D1_ARC_SEGMENT.
type arcSegment struct {
	point         pointF
	width, height float32
	rotation      float32
	sweep, size   uint32
}

// bezierSegment is D2D1_BEZIER_SEGMENT.
type bezierSegment struct{ p1, p2, p3 pointF }

// bitmapProperties1 is D2D1_BITMAP_PROPERTIES1.
type bitmapProperties1 struct {
	format, alphaMode uint32
	dpiX, dpiY        float32
	options           uint32
	colorContext      uintptr
}

// layerParameters1 is D2D1_LAYER_PARAMETERS1.
type layerParameters1 struct {
	contentBounds rectF
	mask          uintptr
	maskAntialias uint32
	maskTransform matrix3x2
	opacity       float32
	opacityBrush  uintptr
	options       uint32
}

// strokeStyleProperties is D2D1_STROKE_STYLE_PROPERTIES.
type strokeStyleProperties struct {
	startCap, endCap, dashCap, lineJoin uint32
	miterLimit                          float32
	dashStyle                           uint32
	dashOffset                          float32
}

// swapChainDesc1 is DXGI_SWAP_CHAIN_DESC1.
type swapChainDesc1 struct {
	width, height uint32
	format        uint32
	stereo        int32
	sampleCount   uint32
	sampleQuality uint32
	bufferUsage   uint32
	bufferCount   uint32
	scaling       uint32
	swapEffect    uint32
	alphaMode     uint32
	flags         uint32
}

// fontMetrics is DWRITE_FONT_METRICS.
type fontMetrics struct {
	designUnitsPerEm       uint16
	ascent, descent        uint16
	lineGap                int16
	capHeight, xHeight     uint16
	underlinePosition      int16
	underlineThickness     uint16
	strikethroughPosition  int16
	strikethroughThickness uint16
}

// textMetrics is DWRITE_TEXT_METRICS.
type textMetrics struct {
	left, top, width, widthWithTrailing, height float32
	layoutWidth, layoutHeight                   float32
	maxBidiDepth, lineCount                     uint32
}

// lineMetrics is DWRITE_LINE_METRICS.
type lineMetrics struct {
	length, trailingWhitespace, newline uint32
	height, baseline                    float32
	trimmed                             int32
}

// hitTestMetrics is DWRITE_HIT_TEST_METRICS.
type hitTestMetrics struct {
	position, length         uint32
	left, top, width, height float32
	bidiLevel                uint32
	isText, isTrimmed        int32
}

// trimming is DWRITE_TRIMMING.
type trimming struct{ granularity, delimiter, delimiterCount uint32 }

// inlineMetrics is DWRITE_INLINE_OBJECT_METRICS.
type inlineMetrics struct {
	width, height, baseline float32
	sideways                int32
}

// overhangMetrics is DWRITE_OVERHANG_METRICS.
type overhangMetrics struct{ left, top, right, bottom float32 }
