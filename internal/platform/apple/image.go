//go:build darwin

package apple

import (
	"context"
	"errors"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/imagedecode"
	"github.com/oneo-me/nsgo/internal/platform/imageload"
)

var (
	cfDataCreate                 func(alloc uintptr, bytes *byte, n int) uintptr
	cfDictionaryGetValue         func(dict, key uintptr) uintptr
	cfNumberGetValue             func(num uintptr, kind int, value unsafe.Pointer) bool
	cgImageGetWidth              func(uintptr) int
	cgImageGetHeight             func(uintptr) int
	cgImageSourceCreateWithData  func(data, options uintptr) uintptr
	cgImageSourceCopyProperties  func(src uintptr, i int, options uintptr) uintptr
	cgImageSourceCreateThumbnail func(src uintptr, i int, options uintptr) uintptr
	cgImageSourceGetCount        func(src uintptr) int
	cgColorSpaceCreateWithName   func(name uintptr) uintptr
	cgDataProviderCreateWithData func(data uintptr) uintptr
	cgImageCreate                func(w, h, bits, pixelBits, row int, space uintptr, info uint32, provider, decode uintptr, interpolate bool, intent int32) uintptr
	srgb                         uintptr
	cfTrue                       uintptr
	// The dictionaries of the animated formats, each with its delay keys
	// (unclamped first) and loop count.
	animatedKeys                  [][4]uintptr
	keyWidth, keyHeight, keyTurn  uintptr
	keyAlways, keyUpright, keyMax uintptr
	keyDecodeNow                  uintptr

	ciOnce    sync.Once
	ciContext objc.ID

	selNew, selRelease    = sel("new"), sel("release")
	selSetFrame           = sel("setFrame:")
	selSetContents        = sel("setContents:")
	selSetContentsGravity = sel("setContentsGravity:")
	selSetContentsScale   = sel("setContentsScale:")
	selSetMask            = sel("setMask:")
	selAddAnimation       = sel("addAnimation:forKey:")
	selNumberWithDouble   = sel("numberWithDouble:")
)

func loadImage(cf, cg uintptr) {
	io := load("/System/Library/Frameworks/ImageIO.framework/ImageIO")
	reg := purego.RegisterLibFunc
	reg(&cfDataCreate, cf, "CFDataCreate")
	reg(&cfDictionaryGetValue, cf, "CFDictionaryGetValue")
	reg(&cfNumberGetValue, cf, "CFNumberGetValue")
	reg(&cgImageGetWidth, cg, "CGImageGetWidth")
	reg(&cgImageGetHeight, cg, "CGImageGetHeight")
	reg(&cgImageSourceCreateWithData, io, "CGImageSourceCreateWithData")
	reg(&cgImageSourceCopyProperties, io, "CGImageSourceCopyPropertiesAtIndex")
	reg(&cgImageSourceCreateThumbnail, io, "CGImageSourceCreateThumbnailAtIndex")
	reg(&cgImageSourceGetCount, io, "CGImageSourceGetCount")
	reg(&cgColorSpaceCreateWithName, cg, "CGColorSpaceCreateWithName")
	reg(&cgDataProviderCreateWithData, cg, "CGDataProviderCreateWithCFData")
	reg(&cgImageCreate, cg, "CGImageCreate")
	srgb = cgColorSpaceCreateWithName(constant(cg, "kCGColorSpaceSRGB"))
	for _, k := range [][4]string{
		{"kCGImagePropertyGIFDictionary", "kCGImagePropertyGIFUnclampedDelayTime", "kCGImagePropertyGIFDelayTime", "kCGImagePropertyGIFLoopCount"},
		{"kCGImagePropertyWebPDictionary", "kCGImagePropertyWebPUnclampedDelayTime", "kCGImagePropertyWebPDelayTime", "kCGImagePropertyWebPLoopCount"},
		{"kCGImagePropertyPNGDictionary", "kCGImagePropertyAPNGUnclampedDelayTime", "kCGImagePropertyAPNGDelayTime", "kCGImagePropertyAPNGLoopCount"},
	} {
		var keys [4]uintptr
		for i, name := range k {
			if sym, err := purego.Dlsym(io, name); err == nil && sym != 0 {
				keys[i] = **(**uintptr)(unsafe.Pointer(&sym))
			}
		}
		if keys[0] != 0 {
			animatedKeys = append(animatedKeys, keys)
		}
	}
	cfTrue = constant(cf, "kCFBooleanTrue")
	keyWidth = constant(io, "kCGImagePropertyPixelWidth")
	keyHeight = constant(io, "kCGImagePropertyPixelHeight")
	keyTurn = constant(io, "kCGImagePropertyOrientation")
	keyAlways = constant(io, "kCGImageSourceCreateThumbnailFromImageAlways")
	keyUpright = constant(io, "kCGImageSourceCreateThumbnailWithTransform")
	keyMax = constant(io, "kCGImageSourceThumbnailMaxPixelSize")
	keyDecodeNow = constant(io, "kCGImageSourceShouldCacheImmediately")
}

// request is how an image view shows what it loads.
type request struct {
	mode  driver.ResizeMode
	view  CGSize  // the view's size in points; zero before layout
	ratio float64 // the screen's pixels per point
	scale float64 // the image's pixels per point at its natural size
	blur  float64 // points
}

// pointsPerPixel returns how many points one pixel of a w×h image covers
// when r's view shows it; 0 when the view has no size yet.
func (r request) pointsPerPixel(w, h float64) float64 {
	if r.view.Width <= 0 || r.view.Height <= 0 || w <= 0 || h <= 0 {
		return 0
	}
	sx, sy := r.view.Width/w, r.view.Height/h
	switch r.mode {
	case driver.Contain:
		return min(sx, sy)
	case driver.Center:
		return 1 / r.scale
	case driver.ScaleDown:
		return min(sx, sy, 1/r.scale)
	}
	return max(sx, sy)
}

// maxPixels returns the longer side to decode a w×h image at so it stays
// sharp in the view, or 0 to decode all of it.
func (r request) maxPixels(w, h float64) float64 {
	ppp := r.pointsPerPixel(w, h)
	if ppp == 0 {
		return 0
	}
	if m := math.Ceil(max(w, h) * ppp * r.ratio); m < max(w, h) {
		return m
	}
	return 0
}

// decoded is a CGImage ImageIO decoded, released by its holder, and the
// source's upright size in pixels. An animated image also has its frames,
// the first being image, each shown for its delay, and its loop count (0
// repeats forever).
type decoded struct {
	image         uintptr
	width, height float64
	scale         float64
	// points is the natural size in points: an SVG's intrinsic size, else
	// the pixels over the scale its name declares.
	points driver.Size
	frames []uintptr
	delays []float64 // seconds
	loops  int
}

// maxFrames bounds the frames of an animated image, as NSH's iOS does.
const maxFrames = 600

// release releases the images d holds.
func (d *decoded) release() {
	Release(d.image)
	for _, f := range d.frames[min(len(d.frames), 1):] {
		Release(f)
	}
}

// animation reads the delays and loop count of an animated source's
// frames; ok is false for a still one.
func animation(src uintptr) (delays []float64, loops int, ok bool) {
	n := min(cgImageSourceGetCount(src), maxFrames)
	if n < 2 {
		return nil, 0, false
	}
	for i := range n {
		delay := 0.1
		props := cgImageSourceCopyProperties(src, i, 0)
		for _, k := range animatedKeys {
			dict := cfDictionaryGetValue(props, k[0])
			if dict == 0 {
				continue
			}
			d := number(dict, k[1])
			if d <= 0 && k[2] != 0 {
				d = number(dict, k[2])
			}
			// Browsers show frames of 10 ms or less for 100 ms.
			if d > 0.01 {
				delay = d
			}
			if i == 0 && k[3] != 0 {
				loops = int(number(dict, k[3]))
			}
			break
		}
		Release(props)
		delays = append(delays, delay)
	}
	return delays, loops, true
}

// decode decodes data upright and at once, at the size r shows it, in
// one of imagedecode's slots; it returns ctx's error if ctx ends first.
func decode(ctx context.Context, data []byte, r request) (decoded, error) {
	if len(data) == 0 {
		return decoded{}, errors.New("apple: empty image data")
	}
	done, err := imagedecode.Wait(ctx)
	if err != nil {
		return decoded{}, err
	}
	defer done()
	if imagedecode.Sniff(data) == imagedecode.SVG {
		return decodeSVG(data, r)
	}
	cfData := cfDataCreate(0, &data[0], len(data))
	defer Release(cfData)
	src := cgImageSourceCreateWithData(cfData, 0)
	if src == 0 {
		return decoded{}, errors.New("apple: unsupported image data")
	}
	defer Release(src)
	props := cgImageSourceCopyProperties(src, 0, 0)
	if props == 0 {
		return decoded{}, errors.New("apple: unsupported image data")
	}
	w, h := number(props, keyWidth), number(props, keyHeight)
	// EXIF orientations 5 to 8 turn the image a quarter.
	if number(props, keyTurn) >= 5 {
		w, h = h, w
	}
	Release(props)
	keys := []uintptr{keyAlways, keyUpright, keyDecodeNow}
	values := []uintptr{cfTrue, cfTrue, cfTrue}
	if m := r.maxPixels(w, h); m > 0 {
		n := float64Number(m)
		defer Release(n)
		keys, values = append(keys, keyMax), append(values, n)
	}
	opts := dictionary(keys, values)
	defer Release(opts)
	img := cgImageSourceCreateThumbnail(src, 0, opts)
	if img == 0 {
		return decoded{}, errors.New("apple: cannot decode the image")
	}
	// An animated image plays unblurred, as NSH's does.
	if delays, loops, ok := animation(src); ok && r.blur <= 0 {
		d := decoded{image: img, width: w, height: h, scale: r.scale, points: pointsOf(w, h, r.scale), frames: []uintptr{img}, loops: loops}
		for i := 1; i < len(delays); i++ {
			f := cgImageSourceCreateThumbnail(src, i, opts)
			if f == 0 {
				break
			}
			d.frames = append(d.frames, f)
		}
		d.delays = delays[:len(d.frames)]
		return d, nil
	}
	if r.blur > 0 {
		dw, dh := float64(cgImageGetWidth(img)), float64(cgImageGetHeight(img))
		radius := r.blur * r.ratio
		if ppp := (request{mode: r.mode, view: r.view, scale: r.scale * dw / w}).pointsPerPixel(dw, dh); ppp > 0 {
			radius = r.blur / ppp
		}
		if out := blur(img, radius); out != 0 {
			Release(img)
			img = out
		}
	}
	return decoded{image: img, width: w, height: h, scale: r.scale, points: pointsOf(w, h, r.scale)}, nil
}

// decodeSVG draws an SVG with Go's rasterizer for the pixels r's view
// shows it at: its shorter side as long as the view's longer side, so no
// resize mode enlarges the raster; at its own size for the screen before
// layout. Its size is the document's, in points.
func decodeSVG(data []byte, r request) (decoded, error) {
	opts := imagedecode.Options{Layout: imagedecode.RGBA, Scale: float32(max(r.ratio, 1))}
	if r.view.Width > 0 && r.view.Height > 0 && r.mode != driver.Center {
		opts.Cover = int(math.Ceil(max(r.view.Width, r.view.Height) * max(r.ratio, 1)))
	}
	img, err := imagedecode.Decode(data, opts)
	if err != nil {
		return decoded{}, err
	}
	pix := img.Frames[0].Pix
	cfData := cfDataCreate(0, &pix[0], len(pix))
	defer Release(cfData)
	provider := cgDataProviderCreateWithData(cfData)
	defer Release(provider)
	const premultipliedLast = 1
	cg := cgImageCreate(img.Width, img.Height, 8, 32, img.Width*4, srgb, premultipliedLast, provider, 0, true, 0)
	if cg == 0 {
		return decoded{}, errors.New("apple: cannot make the SVG's image")
	}
	// Points per pixel of the raster: the contents scale that shows it at
	// the document's size.
	scale := float64(img.Width) / max(float64(img.Size.Width), 1)
	if r.blur > 0 {
		if out := blur(cg, r.blur*scale); out != 0 {
			Release(cg)
			cg = out
		}
	}
	return decoded{image: cg, width: math.Round(float64(img.Size.Width)), height: math.Round(float64(img.Size.Height)),
		scale: scale, points: img.Size}, nil
}

// pointsOf returns the natural size of a w×h image whose pixels per point
// are scale.
func pointsOf(w, h, scale float64) driver.Size {
	if scale <= 0 {
		scale = 1
	}
	return driver.Size{Width: driver.Points(w / scale), Height: driver.Points(h / scale)}
}

// ImageInfo answers an ItemImageInfo on a goroutine of its own: it loads
// the source, keeping its bytes for the views that show it next, and
// dispatches EventImageInfo with the size ImageIO reads, upright, or an
// SVG's in whole points.
func ImageInfo(it *driver.MountItem, dispatch func(driver.Event)) {
	item := *it
	go func() { dispatch(imageload.Info(&item, imageSize)) }()
}

// imageSize returns the size EventLoad reports for data.
func imageSize(data []byte) (driver.Size, error) {
	if len(data) == 0 {
		return driver.Size{}, errors.New("apple: empty image data")
	}
	if imagedecode.Sniff(data) == imagedecode.SVG {
		return imagedecode.Size(data)
	}
	cfData := cfDataCreate(0, &data[0], len(data))
	defer Release(cfData)
	src := cgImageSourceCreateWithData(cfData, 0)
	if src == 0 {
		return driver.Size{}, errors.New("apple: unsupported image data")
	}
	defer Release(src)
	props := cgImageSourceCopyProperties(src, 0, 0)
	if props == 0 {
		return driver.Size{}, errors.New("apple: unsupported image data")
	}
	defer Release(props)
	w, h := number(props, keyWidth), number(props, keyHeight)
	if number(props, keyTurn) >= 5 {
		w, h = h, w
	}
	if w <= 0 || h <= 0 {
		return driver.Size{}, errors.New("apple: image has no pixels")
	}
	return driver.Size{Width: driver.Points(w), Height: driver.Points(h)}, nil
}

func number(dict, key uintptr) float64 {
	var v float64
	if n := cfDictionaryGetValue(dict, key); n != 0 {
		const float64Type = 6
		cfNumberGetValue(n, float64Type, unsafe.Pointer(&v))
	}
	return v
}

// blur returns a new CGImage of img blurred by radius pixels with
// CoreImage, or 0.
func blur(img uintptr, radius float64) (out uintptr) {
	ciOnce.Do(func() {
		load("/System/Library/Frameworks/CoreImage.framework/CoreImage")
		withPool(func() {
			ciContext = objc.ID(objc.GetClass("CIContext")).Send(sel("contextWithOptions:"), objc.ID(0)).Send(sel("retain"))
		})
	})
	withPool(func() {
		in := objc.ID(objc.GetClass("CIImage")).Send(sel("imageWithCGImage:"), img)
		extent := objc.Send[CGRect](in, sel("extent"))
		f := objc.ID(objc.GetClass("CIFilter")).Send(sel("filterWithName:"), NSString("CIGaussianBlur"))
		f.Send(sel("setValue:forKey:"), in.Send(sel("imageByClampingToExtent")), NSString("inputImage"))
		f.Send(sel("setValue:forKey:"), objc.ID(objc.GetClass("NSNumber")).Send(selNumberWithDouble, radius), NSString("inputRadius"))
		out = objc.Send[uintptr](ciContext, sel("createCGImage:fromRect:"), f.Send(sel("outputImage")), extent)
	})
	return out
}

// uriScale returns the pixels per point an "@2x"-style name declares.
func uriScale(uri string) float64 {
	base := uri
	if dot := strings.LastIndexByte(uri, '.'); dot > strings.LastIndexByte(uri, '/') {
		base = uri[:dot]
	}
	at := strings.LastIndexByte(base, '@')
	if at < 0 || !strings.HasSuffix(base, "x") {
		return 1
	}
	if f, err := strconv.ParseFloat(base[at+1:len(base)-1], 64); err == nil && f > 0 {
		return f
	}
	return 1
}

func hasSource(s driver.ImageSource) bool { return s.URI != "" || len(s.Data) > 0 }

func sameSource(a, b driver.ImageSource) bool {
	return a.URI == b.URI && len(a.Data) == len(b.Data) && (len(a.Data) == 0 || &a.Data[0] == &b.Data[0])
}

// ImageView shows an image view's source, and its placeholder until the
// source arrives, in a picture layer framed at the fitted rectangle
// inside a clipping layer it adds to the view's own, as NSH's does (the
// tint and its mask inside the picture). It loads them with imageload and
// decodes them with ImageIO on goroutines, at the size the view shows
// them. Its methods run on the main thread.
type ImageView struct {
	// Dispatch reports an event to the engine if the view listens for it.
	Dispatch func(driver.Event)

	layer, picture, mask objc.ID
	config               driver.ImageConfig
	tint                 driver.Color
	corners              driver.ViewStyle // its corners: CornerRadius, Corners, ContinuousCorners
	bounds               CGSize
	ratio                float64

	image   uintptr // the CGImage shown, retained
	shown   decoded // what image was decoded from
	source  bool    // whether image is the source rather than the placeholder
	reduced bool    // whether image is smaller than the source

	load    int // numbers loads; an older load's results are dropped
	cancel  context.CancelFunc
	pending bool // a load waits for Start
	quiet   bool // the pending load decodes the source shown again, larger
	closed  bool
}

// NewImageView adds an image layer to host, a view's layer.
func NewImageView(host objc.ID) *ImageView {
	layer := objc.ID(objc.GetClass("CALayer"))
	iv := &ImageView{layer: layer.Send(selNew), picture: layer.Send(selNew)}
	iv.layer.Send(selSetMasksToBounds, true)
	iv.layer.Send(sel("addSublayer:"), iv.picture)
	iv.picture.Send(selRelease)
	host.Send(sel("addSublayer:"), iv.layer)
	iv.layer.Send(selRelease)
	return iv
}

// Configure sets the view's source and look. Start loads what changed.
func (iv *ImageView) Configure(c *driver.ImageConfig) {
	old := iv.config
	iv.config = *c
	if iv.load == 0 || !sameSource(old.Source, c.Source) || !sameSource(old.Placeholder, c.Placeholder) ||
		old.BlurRadius != c.BlurRadius {
		iv.pending, iv.quiet = true, false
	} else if old.ResizeMode != c.ResizeMode && iv.source && iv.reduced {
		iv.pending, iv.quiet = true, true // the mode may show more pixels
	}
	iv.render(0)
}

// SetStyle applies what of a view's style the image layer draws: the
// tint and the corners.
func (iv *ImageView) SetStyle(vs *driver.ViewStyle) {
	iv.corners = driver.ViewStyle{CornerRadius: vs.CornerRadius, Corners: vs.Corners, ContinuousCorners: vs.ContinuousCorners}
	iv.setCorners()
	if vs.TintColor != iv.tint {
		iv.tint = vs.TintColor
		iv.render(0)
	}
}

// setCorners clips the image to the view's corners where they share a
// radius, as the view's layer clips them.
func (iv *ImageView) setCorners() {
	w, h := iv.bounds.Width, iv.bounds.Height
	var r driver.Points
	var masked uint
	if radii := iv.corners.Radii(driver.Size{Width: driver.Points(w), Height: driver.Points(h)}); radii.Uniform() {
		r, masked = cornersOf(radii)
	}
	WithoutAnimation(func() { setCorners(iv.layer, r, masked, iv.corners.ContinuousCorners, w, h) })
}

// Layout fits the image to the view's new size, decoding the source
// again when the view outgrows the size it was decoded at.
func (iv *ImageView) Layout(size CGSize) {
	iv.bounds = size
	iv.setCorners()
	iv.render(0)
	if !iv.source || !iv.reduced || iv.pending {
		return
	}
	r, w, h := iv.request(iv.ratio), iv.shown.width, iv.shown.height
	if r.pointsPerPixel(w, h) == 0 {
		return
	}
	// maxPixels is 0 when the view needs all of the source.
	have := max(float64(cgImageGetWidth(iv.image)), float64(cgImageGetHeight(iv.image)))
	if m := r.maxPixels(w, h); m == 0 || m > have*1.25 {
		iv.pending, iv.quiet = true, true
	}
}

func (iv *ImageView) request(ratio float64) request {
	return request{mode: iv.config.ResizeMode, view: iv.bounds, ratio: ratio,
		scale: uriScale(iv.config.Source.URI), blur: float64(iv.config.BlurRadius)}
}

// Start loads what Configure or Layout changed, for a screen of ratio
// pixels per point. Backends call it once a batch has sized the view.
func (iv *ImageView) Start(ratio float64) {
	if !iv.pending || iv.closed {
		return
	}
	if ratio <= 0 {
		ratio = 1
	}
	iv.ratio = ratio
	iv.pending = false
	if iv.cancel != nil {
		iv.cancel()
	}
	iv.load++
	n := iv.load
	ctx, cancel := context.WithCancel(context.Background())
	iv.cancel = cancel
	c := iv.config
	r := iv.request(ratio)
	quiet := iv.quiet
	iv.quiet = false
	if !quiet {
		iv.show(decoded{}, false, 0)
		if hasSource(c.Placeholder) {
			pr := r
			pr.scale = uriScale(c.Placeholder.URI)
			go iv.fetch(ctx, n, c.Placeholder, pr, false, true)
		}
	}
	if !hasSource(c.Source) {
		return
	}
	if !quiet {
		iv.Dispatch(driver.Event{Kind: driver.EventLoadStart})
	}
	go iv.fetch(ctx, n, c.Source, r, true, quiet)
}

// fetch loads and decodes src on a goroutine of its own, then shows it on
// the main thread unless a later load replaced this one, numbered n.
func (iv *ImageView) fetch(ctx context.Context, n int, src driver.ImageSource, r request, source, quiet bool) {
	var progress imageload.Progress
	if !quiet {
		var last time.Time
		progress = func(loaded, total int64) {
			if now := time.Now(); now.Sub(last) >= 100*time.Millisecond || loaded == total {
				last = now
				OnMain(func() {
					if iv.load == n {
						iv.Dispatch(driver.Event{Kind: driver.EventProgress, Loaded: loaded, Total: total})
					}
				})
			}
		}
	}
	data, err := imageload.Load(ctx, src, progress)
	if ctx.Err() != nil {
		return
	}
	var img decoded
	if err == nil {
		img, err = decode(ctx, data, r)
	}
	OnMain(func() {
		if iv.load != n || (!source && iv.source) {
			img.release()
			return
		}
		switch {
		case !source:
			if err == nil {
				iv.show(img, false, 0)
			}
		case err != nil:
			iv.Dispatch(driver.Event{Kind: driver.EventError, Text: err.Error()})
			iv.Dispatch(driver.Event{Kind: driver.EventLoadEnd})
		case quiet:
			iv.show(img, true, 0)
		default:
			iv.show(img, true, iv.config.FadeDuration)
			iv.Dispatch(driver.Event{Kind: driver.EventLoad, Size: driver.Size{Width: driver.Points(img.width), Height: driver.Points(img.height)}})
			iv.Dispatch(driver.Event{Kind: driver.EventLoadEnd})
		}
	})
}

func (iv *ImageView) show(img decoded, source bool, fade time.Duration) {
	iv.shown.release()
	iv.image, iv.shown, iv.source = img.image, img, source && img.image != 0
	iv.reduced = img.image != 0 && float64(cgImageGetWidth(img.image)) < img.width
	iv.render(fade)
}

// fitted returns where the picture goes in the view: core's fit of the
// shown image's natural size, or the size its source declares, by the
// resize mode and position.
func (iv *ImageView) fitted() CGRect {
	natural := iv.shown.points
	src := iv.config.Placeholder
	if iv.source {
		src = iv.config.Source
	}
	if size := src.Size; size.Width > 0 && size.Height > 0 {
		natural = size
	}
	box := driver.Size{Width: driver.Points(iv.bounds.Width), Height: driver.Points(iv.bounds.Height)}
	b := iv.config.ResizeMode.Fit(natural, box, iv.config.Position)
	return Rect(float64(b.Origin.X), float64(b.Origin.Y), float64(b.Size.Width), float64(b.Size.Height))
}

// render shows the image in the picture layer: as it is, or as a template
// the tint fills through a mask of the image's alpha.
func (iv *ImageView) render(fade time.Duration) {
	WithoutAnimation(func() {
		iv.layer.Send(selSetFrame, Rect(0, 0, iv.bounds.Width, iv.bounds.Height))
		fit := iv.fitted()
		iv.picture.Send(selSetFrame, fit)
		target := iv.picture
		if iv.tint == 0 {
			iv.picture.Send(selSetBackgroundColor, uintptr(0))
			iv.picture.Send(selSetMask, objc.ID(0))
		} else {
			if iv.mask == 0 {
				iv.mask = objc.ID(objc.GetClass("CALayer")).Send(selNew)
			}
			iv.mask.Send(selSetFrame, Rect(0, 0, fit.Size.Width, fit.Size.Height))
			color := CGColor(iv.tint)
			iv.picture.Send(selSetBackgroundColor, color)
			Release(color)
			iv.picture.Send(selSetContents, uintptr(0))
			iv.picture.Send(selSetMask, iv.mask)
			target = iv.mask
		}
		if fade > 0 {
			t := objc.ID(objc.GetClass("CATransition")).Send(sel("animation"))
			t.Send(sel("setType:"), NSString("fade"))
			t.Send(sel("setDuration:"), fade.Seconds())
			target.Send(selAddAnimation, t, NSString("contents"))
		}
		target.Send(selSetContents, iv.image)
		target.Send(selRemoveAnimationForKey, NSString("nsgoFrames"))
		if len(iv.shown.frames) > 1 && iv.tint == 0 {
			target.Send(selAddAnimation, frameAnimation(&iv.shown), NSString("nsgoFrames"))
		}
		target.Send(selSetContentsGravity, NSString("resize"))
		scale := iv.shown.scale
		if scale <= 0 {
			scale = 1
		}
		target.Send(selSetContentsScale, scale)
	})
}

// Close stops loading and lets the image go; the view is being deleted.
func (iv *ImageView) Close() {
	if iv.cancel != nil {
		iv.cancel()
	}
	iv.load++
	iv.closed = true
	iv.shown.release()
	iv.image, iv.shown = 0, decoded{}
	if iv.mask != 0 {
		iv.mask.Send(selRelease)
		iv.mask = 0
	}
}

var selRemoveAnimationForKey = sel("removeAnimationForKey:")

// frameAnimation plays an animated image's frames on its layer's contents,
// each for its delay: a discrete keyframe animation, which Core Animation
// runs without the app.
func frameAnimation(d *decoded) objc.ID {
	values := objc.ID(objc.GetClass("NSMutableArray")).Send(sel("array"))
	times := objc.ID(objc.GetClass("NSMutableArray")).Send(sel("array"))
	number := objc.ID(objc.GetClass("NSNumber"))
	total := 0.0
	for _, delay := range d.delays {
		total += delay
	}
	at := 0.0
	for i, f := range d.frames {
		values.Send(sel("addObject:"), f)
		times.Send(sel("addObject:"), number.Send(selNumberWithDouble, at/total))
		at += d.delays[i]
	}
	times.Send(sel("addObject:"), number.Send(selNumberWithDouble, 1.0))
	anim := objc.ID(objc.GetClass("CAKeyframeAnimation")).Send(sel("animationWithKeyPath:"), NSString("contents"))
	anim.Send(sel("setValues:"), values)
	anim.Send(sel("setKeyTimes:"), times)
	anim.Send(sel("setCalculationMode:"), NSString("discrete"))
	anim.Send(sel("setDuration:"), total)
	repeat := float32(math.Inf(1))
	if d.loops > 0 {
		repeat = float32(d.loops)
	}
	anim.Send(sel("setRepeatCount:"), repeat)
	// Keep the last frame once a counted loop ends.
	anim.Send(sel("setFillMode:"), NSString("forwards"))
	anim.Send(sel("setRemovedOnCompletion:"), false)
	return anim
}

// AnimateIn presents a modal's layer as a asks: sliding up from distance
// points below, or fading in. Reduced motion fades instead of sliding.
func AnimateIn(layer objc.ID, a driver.ModalAnimation, distance float64, reduceMotion bool) {
	if a == driver.ModalSlide && reduceMotion {
		a = driver.ModalFade
	}
	number := objc.ID(objc.GetClass("NSNumber"))
	basic := objc.ID(objc.GetClass("CABasicAnimation"))
	var anim objc.ID
	switch a {
	case driver.ModalSlide:
		anim = basic.Send(sel("animationWithKeyPath:"), NSString("transform.translation.y"))
		anim.Send(sel("setFromValue:"), number.Send(selNumberWithDouble, distance))
		anim.Send(sel("setDuration:"), 0.3)
		anim.Send(sel("setTimingFunction:"), objc.ID(objc.GetClass("CAMediaTimingFunction")).Send(
			sel("functionWithName:"), NSString("easeOut")))
	case driver.ModalFade:
		anim = basic.Send(sel("animationWithKeyPath:"), NSString("opacity"))
		anim.Send(sel("setFromValue:"), number.Send(selNumberWithDouble, 0.0))
		anim.Send(sel("setDuration:"), 0.2)
	default:
		return
	}
	layer.Send(selAddAnimation, anim, NSString("nsgoPresent"))
}
