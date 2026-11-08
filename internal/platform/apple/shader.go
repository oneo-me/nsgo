//go:build darwin

package apple

import (
	"fmt"
	"log"
	"math"
	"runtime"
	"strings"
	"structs"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/shaderir"
	"github.com/oneo-me/nsgo/internal/shaderir/msl"
)

// ShaderLayer draws a shader view with Metal: a CAMetalLayer the view's
// layer holds below its children, drawn when the uniforms, the size or the
// animated lanes change, never on a timer of its own. Metal loads with the
// first shader, and each program compiles once for the process on a
// goroutine of its own, as Metal takes a tenth of a second for a library;
// the layer draws once it is ready.
//
// The main thread only writes the uniforms (Draw); a render goroutine of
// the layer's own waits for a drawable, encodes and presents. Waiting for
// a drawable blocks until the display frees one, a whole refresh when
// frames come faster, so the main thread must never wait for it.
type ShaderLayer struct {
	// Layer is the CAMetalLayer.
	Layer objc.ID
	// Ready is called when the program compiled after Set, to draw.
	Ready func()

	shader *driver.Shader
	pipe   *metalPipeline
	w, h   float64 // points
	scale  float64
	lanes  []float64
	buf    []byte
	render *shaderRender
	// textures are the images' Metal textures, made once each loads, and
	// sizes their pixels.
	textures []objc.ID
	sizes    [][2]float64
	sources  []driver.ImageSource
	// pictures are what a WGSL program reads in place of images: content,
	// backdrop, from and to (SetPictures).
	pictures [4]*Picture
}

// SetPictures gives a WGSL program the pictures it reads; a nil one, or
// one not taken yet, reads as transparent.
func (l *ShaderLayer) SetPictures(p [4]*Picture) { l.pictures = p }

// Drawable reports whether the layer has a program ready to draw.
func (l *ShaderLayer) Drawable() bool {
	return l.shader != nil && l.pipe != nil && l.pipe.state != 0
}

// shaderRender is what the main thread hands a layer's render goroutine:
// the latest frame's pipeline state and uniforms, which replace a frame
// not drawn yet.
type shaderRender struct {
	layer    objc.ID
	wake     chan struct{}
	mu       sync.Mutex
	state    objc.ID
	buf      []byte
	textures []objc.ID // retained for the frame
	fresh    bool
}

// metalPipeline is a program compiled for Metal: its pipeline state, 0
// until it is ready or when it failed, and where its uniforms' lanes lie.
type metalPipeline struct {
	state    objc.ID
	ready    bool
	offsets  []int
	types    []shaderir.BasicType
	size     int
	textures int // the images it reads
	waiting  []*ShaderLayer
}

// mtlClearColor is Metal's MTLClearColor.
type mtlClearColor struct {
	_          structs.HostLayout
	R, G, B, A float64
}

const (
	mtlPixelFormatBGRA8Unorm = 80
	mtlLoadActionClear       = 2
	mtlStoreActionStore      = 1
	mtlPrimitiveTypeTriangle = 3
	// mtlMaxBytes is the most setFragmentBytes takes; larger uniforms go
	// in a buffer.
	mtlMaxBytes = 4096
)

var (
	metalTried bool
	mtlDevice  objc.ID
	mtlQueue   objc.ID
	// pipelines are the programs compiled so far, by the program, which
	// apps keep for their lives.
	pipelines = map[*driver.ShaderProgram]*metalPipeline{}

	selNextDrawable             = sel("nextDrawable")
	selColorAttachments         = sel("colorAttachments")
	selObjectAtIndexedSubscript = sel("objectAtIndexedSubscript:")
)

// loadMetal loads Metal and makes the device and queue every shader view
// shares, once; it reports whether the machine has a Metal device.
func loadMetal() bool {
	if metalTried {
		return mtlDevice != 0
	}
	metalTried = true
	lib, err := purego.Dlopen("/System/Library/Frameworks/Metal.framework/Metal", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return false
	}
	var create func() objc.ID
	purego.RegisterLibFunc(&create, lib, "MTLCreateSystemDefaultDevice")
	mtlDevice = create()
	if mtlDevice == 0 {
		return false
	}
	mtlQueue = mtlDevice.Send(sel("newCommandQueue"))
	return true
}

// NewShaderLayer returns a shader view's layer at scale pixels a point,
// or nil where the machine has no Metal device: the view stays empty.
func NewShaderLayer(scale float64) *ShaderLayer {
	if !loadMetal() {
		return nil
	}
	layer := objc.ID(objc.GetClass("CAMetalLayer")).Send(selNew)
	layer.Send(sel("setDevice:"), mtlDevice)
	layer.Send(sel("setPixelFormat:"), uint(mtlPixelFormatBGRA8Unorm))
	layer.Send(sel("setFramebufferOnly:"), true)
	layer.Send(sel("setOpaque:"), false)
	// Shaders compute sRGB colors, as the views' styles give them.
	layer.Send(sel("setColorspace:"), srgb)
	layer.Send(sel("setContentsScale:"), scale)
	r := &shaderRender{layer: layer, wake: make(chan struct{}, 1)}
	go r.loop()
	return &ShaderLayer{Layer: layer, scale: scale, render: r}
}

// Set gives the layer what it draws, compiling its program the first time
// a view shows it.
func (l *ShaderLayer) Set(sh *driver.Shader) {
	l.shader = sh
	l.setImages(sh.Images)
	p := pipelines[sh.Program]
	if p == nil {
		p = compileMetal(sh.Program)
		pipelines[sh.Program] = p
	}
	if l.pipe != p {
		l.unwait()
		l.pipe = p
		if !p.ready {
			p.waiting = append(p.waiting, l)
		}
	}
}

// Resize sizes the layer's drawable for w by h points at scale.
func (l *ShaderLayer) Resize(w, h, scale float64) {
	if w == l.w && h == l.h && scale == l.scale {
		return
	}
	l.w, l.h, l.scale = w, h, scale
	l.Layer.Send(sel("setContentsScale:"), scale)
	l.Layer.Send(sel("setDrawableSize:"), CGSize{Width: math.Ceil(w * scale), Height: math.Ceil(h * scale)})
}

// Draw has the render goroutine draw the shader with its uniforms,
// animated writing the lanes animated nodes drive over them. It never
// waits.
func (l *ShaderLayer) Draw(animated func(lanes []float64)) {
	p := l.pipe
	if l.shader == nil || p == nil || p.state == 0 || l.w <= 0 || l.h <= 0 {
		return
	}
	bridge := 3 + 2*p.textures
	n := bridge + len(l.shader.Lanes)
	l.lanes = append(l.lanes[:0], make([]float64, n)...)
	copy(l.lanes[bridge:], l.shader.Lanes)
	if animated != nil {
		animated(l.lanes[bridge:])
	}
	l.lanes[0], l.lanes[1], l.lanes[2] = l.w, l.h, l.scale
	textures := make([]objc.ID, p.textures)
	for i := range p.textures {
		textures[i] = blankTexture()
		if l.shader.Program.WGSL {
			if pic := l.pictures[i]; pic != nil && pic.tex != 0 {
				textures[i] = pic.tex
				l.lanes[3+2*i], l.lanes[4+2*i] = float64(pic.w), float64(pic.h)
			}
		} else if i < len(l.textures) && l.textures[i] != 0 {
			textures[i] = l.textures[i]
			l.lanes[3+2*i], l.lanes[4+2*i] = l.sizes[i][0], l.sizes[i][1]
		}
		textures[i].Send(sel("retain"))
	}
	if cap(l.buf) < p.size {
		l.buf = make([]byte, p.size)
	}
	l.buf = l.buf[:p.size]
	clear(l.buf)
	shaderir.PutLanes(l.buf, p.offsets, p.types, l.lanes, 1)

	r := l.render
	r.mu.Lock()
	for _, t := range r.textures {
		t.Send(selRelease)
	}
	r.state, r.buf, r.textures, r.fresh = p.state, append(r.buf[:0], l.buf...), textures, true
	r.mu.Unlock()
	select {
	case r.wake <- struct{}{}:
	default: // a wake is pending, and takes this frame
	}
}

// loop draws the frames the main thread hands over, the latest each time
// a drawable is free, until the layer is released, then lets the layer go.
func (r *shaderRender) loop() {
	runtime.LockOSThread()
	defer r.layer.Send(selRelease)
	var buf []byte
	for range r.wake {
		pool := poolPush()
		drawable := r.layer.Send(selNextDrawable)
		r.mu.Lock()
		state, fresh, textures := r.state, r.fresh, r.textures
		buf = append(buf[:0], r.buf...)
		r.fresh, r.textures = false, nil
		r.mu.Unlock()
		if drawable != 0 && fresh {
			encodeShader(drawable, state, buf, textures)
		}
		for _, t := range textures {
			t.Send(selRelease)
		}
		poolPop(pool)
	}
}

// encodeShader draws state with the uniforms in buf into drawable and
// presents it.
func encodeShader(drawable, state objc.ID, buf []byte, textures []objc.ID) {
	pass := objc.ID(objc.GetClass("MTLRenderPassDescriptor")).Send(sel("renderPassDescriptor"))
	att := pass.Send(selColorAttachments).Send(selObjectAtIndexedSubscript, uint(0))
	att.Send(sel("setTexture:"), drawable.Send(sel("texture")))
	att.Send(sel("setLoadAction:"), uint(mtlLoadActionClear))
	att.Send(sel("setStoreAction:"), uint(mtlStoreActionStore))
	att.Send(sel("setClearColor:"), mtlClearColor{})
	cb := mtlQueue.Send(sel("commandBuffer"))
	enc := cb.Send(sel("renderCommandEncoderWithDescriptor:"), pass)
	enc.Send(sel("setRenderPipelineState:"), state)
	var buffer objc.ID
	if len(buf) <= mtlMaxBytes {
		enc.Send(sel("setFragmentBytes:length:atIndex:"), unsafe.Pointer(&buf[0]), uint(len(buf)), uint(0))
	} else {
		buffer = mtlDevice.Send(sel("newBufferWithBytes:length:options:"), unsafe.Pointer(&buf[0]), uint(len(buf)), uint(0))
		enc.Send(sel("setFragmentBuffer:offset:atIndex:"), buffer, uint(0), uint(0))
	}
	for i, t := range textures {
		enc.Send(sel("setFragmentTexture:atIndex:"), t, uint(i))
	}
	enc.Send(sel("drawPrimitives:vertexStart:vertexCount:"), uint(mtlPrimitiveTypeTriangle), uint(0), uint(3))
	enc.Send(sel("endEncoding"))
	cb.Send(sel("presentDrawable:"), drawable)
	cb.Send(sel("commit"))
	if buffer != 0 {
		buffer.Send(selRelease)
	}
}

// Release lets the layer go, with the view holding it, once its render
// goroutine finished the frame it draws.
func (l *ShaderLayer) Release() {
	l.unwait()
	l.setImages(nil)
	close(l.render.wake)
	l.Layer = 0
}

// setImages loads the images the shader reads, drawing again as each
// arrives, and lets those it no longer reads go.
func (l *ShaderLayer) setImages(images []driver.ImageSource) {
	if len(images) == len(l.sources) {
		same := true
		for i := range images {
			same = same && images[i].Same(l.sources[i])
		}
		if same {
			return
		}
	}
	for _, t := range l.textures {
		if t != 0 {
			t.Send(selRelease)
		}
	}
	l.sources = images
	l.textures, l.sizes = make([]objc.ID, len(images)), make([][2]float64, len(images))
	for i, src := range images {
		l.loadImage(i, src)
	}
}

// loadImage makes image i's texture once it loads.
func (l *ShaderLayer) loadImage(i int, src driver.ImageSource) {
	var try func()
	try = func() {
		if i >= len(l.sources) || !l.sources[i].Same(src) || l.textures[i] != 0 {
			return
		}
		img := canvasImage(src, CanvasHost{Ratio: l.scale, Redraw: try, Owner: l})
		if img == nil || img.image == 0 {
			return
		}
		w, h := int(cgImageGetWidth(img.image)), int(cgImageGetHeight(img.image))
		if t := textureOf(img.image, w, h); t != 0 {
			l.textures[i], l.sizes[i] = t, [2]float64{float64(w), float64(h)}
			if l.Ready != nil {
				l.Ready()
			}
		}
	}
	try()
}

var (
	cgBitmapContextCreate  func(data uintptr, w, h, bits, row uintptr, space uintptr, info uint32) uintptr
	cgBitmapContextGetData func(ctx uintptr) uintptr
	bitmapsLoaded          sync.Once
	blank                  objc.ID
)

// mtlRegion is MTLRegion: an origin and a size, three each.
type mtlRegion struct {
	_                structs.HostLayout
	X, Y, Z, W, H, D uint
}

func loadBitmaps() {
	cg := load("/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics")
	purego.RegisterLibFunc(&cgBitmapContextCreate, cg, "CGBitmapContextCreate")
	purego.RegisterLibFunc(&cgBitmapContextGetData, cg, "CGBitmapContextGetData")
}

// textureOf returns image's pixels, w by h, as a Metal texture of
// premultiplied BGRA, top row first, or 0.
func textureOf(image uintptr, w, h int) objc.ID {
	if w <= 0 || h <= 0 {
		return 0
	}
	bitmapsLoaded.Do(loadBitmaps)
	const premultipliedFirst, byteOrder32Little = 2, 2 << 12
	ctx := cgBitmapContextCreate(0, uintptr(w), uintptr(h), 8, uintptr(w*4), srgb, premultipliedFirst|byteOrder32Little)
	if ctx == 0 {
		return 0
	}
	defer Release(ctx)
	cgContextDrawImage(ctx, Rect(0, 0, float64(w), float64(h)), image)
	desc := objc.ID(objc.GetClass("MTLTextureDescriptor")).Send(
		sel("texture2DDescriptorWithPixelFormat:width:height:mipmapped:"), uint(mtlPixelFormatBGRA8Unorm), uint(w), uint(h), false)
	t := mtlDevice.Send(sel("newTextureWithDescriptor:"), desc)
	if t == 0 {
		return 0
	}
	t.Send(sel("replaceRegion:mipmapLevel:withBytes:bytesPerRow:"), mtlRegion{W: uint(w), H: uint(h), D: 1}, uint(0),
		cgBitmapContextGetData(ctx), uint(w*4))
	return t
}

// blankTexture is a transparent pixel, which an image not loaded reads.
func blankTexture() objc.ID {
	if blank == 0 {
		desc := objc.ID(objc.GetClass("MTLTextureDescriptor")).Send(
			sel("texture2DDescriptorWithPixelFormat:width:height:mipmapped:"), uint(mtlPixelFormatBGRA8Unorm), uint(1), uint(1), false)
		blank = mtlDevice.Send(sel("newTextureWithDescriptor:"), desc)
		var px [4]byte
		blank.Send(sel("replaceRegion:mipmapLevel:withBytes:bytesPerRow:"), mtlRegion{W: 1, H: 1, D: 1}, uint(0),
			unsafe.Pointer(&px[0]), uint(4))
	}
	return blank
}

func (l *ShaderLayer) unwait() {
	if p := l.pipe; p != nil {
		for i, w := range p.waiting {
			if w == l {
				p.waiting = append(p.waiting[:i], p.waiting[i+1:]...)
				break
			}
		}
	}
}

// metalMain is what the program's fragment function becomes the
// fragment stage of: a triangle covering the drawable, and the position
// in points from the pixel's; metalTextures adds the textures of a
// program that reads images.
const metalMain = `
struct NsgoVertexOut {
	float4 position [[position]];
};

vertex NsgoVertexOut nsgoVertex(uint vid [[vertex_id]]) {
	float2 uv = float2((vid << 1) & 2, vid & 2);
	NsgoVertexOut out;
	out.position = float4(uv * float2(2.0, -2.0) + float2(-1.0, 1.0), 0.0, 1.0);
	return out;
}

fragment float4 nsgoFragment(NsgoVertexOut in [[stage_in]], constant Uniforms& uniforms [[buffer(0)]]{{params}}) {
	return nsgoShader(uniforms{{args}}, in.position, in.position.xy / uniforms.U1);
}
`

// metalStage is metalMain for a program reading n images.
func metalStage(n int) string {
	var params, args strings.Builder
	for i := range n {
		fmt.Fprintf(&params, ", texture2d<float> T%[1]d [[texture(%[1]d)]]", i)
		fmt.Fprintf(&args, ", T%d", i)
	}
	return strings.NewReplacer("{{params}}", params.String(), "{{args}}", args.String()).Replace(metalMain)
}

// compileMetal translates prog for Metal and starts compiling it on a
// goroutine, which hands the pipeline state to the main thread and draws
// the layers waiting for it.
func compileMetal(prog *driver.ShaderProgram) *metalPipeline {
	p := &metalPipeline{}
	ir := prog.IR
	p.offsets, p.size = msl.UniformLanes(ir)
	p.types = ir.LaneTypes()
	p.textures = ir.TextureCount
	src := msl.Fragment(ir, "nsgoShader") + metalStage(ir.TextureCount)
	go withPool(func() {
		state, err := metalPipelineState(src)
		if err != "" {
			log.Printf("nsgo: shader: Metal: %s", err)
		}
		OnMain(func() {
			p.state, p.ready = state, true
			waiting := p.waiting
			p.waiting = nil
			for _, l := range waiting {
				if l.Ready != nil {
					l.Ready()
				}
			}
		})
	})
	return p
}

// metalPipelineState compiles src into a pipeline state, or returns
// Metal's error.
func metalPipelineState(src string) (objc.ID, string) {
	var errObj objc.ID
	lib := mtlDevice.Send(sel("newLibraryWithSource:options:error:"), NSString(src), objc.ID(0), unsafe.Pointer(&errObj))
	if lib == 0 {
		return 0, errorText(errObj)
	}
	defer lib.Send(selRelease)
	vertex := lib.Send(sel("newFunctionWithName:"), NSString("nsgoVertex"))
	fragment := lib.Send(sel("newFunctionWithName:"), NSString("nsgoFragment"))
	defer vertex.Send(selRelease)
	defer fragment.Send(selRelease)
	desc := objc.ID(objc.GetClass("MTLRenderPipelineDescriptor")).Send(selNew)
	defer desc.Send(selRelease)
	desc.Send(sel("setVertexFunction:"), vertex)
	desc.Send(sel("setFragmentFunction:"), fragment)
	desc.Send(selColorAttachments).Send(selObjectAtIndexedSubscript, uint(0)).Send(sel("setPixelFormat:"), uint(mtlPixelFormatBGRA8Unorm))
	errObj = 0
	state := mtlDevice.Send(sel("newRenderPipelineStateWithDescriptor:error:"), desc, unsafe.Pointer(&errObj))
	if state == 0 {
		return 0, errorText(errObj)
	}
	return state, ""
}

func errorText(err objc.ID) string {
	if err == 0 {
		return "unknown error"
	}
	return GoString(err.Send(sel("localizedDescription")))
}
