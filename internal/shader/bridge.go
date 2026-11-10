package shader

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"

	"github.com/oneo-me/nsgo/internal/shaderir"
)

// Entry is the name of a shader's entry point, which returns the color
// of a pixel, premultiplied:
//
//	func Fragment(dst vec4, pos vec2) vec4
//
// dst is the pixel's center in the shader's view, in its pixels from the
// top left corner, and pos the same point in points. It may take dst
// alone, or nothing.
const Entry = "Fragment"

// The uniform variables the bridge declares, which platforms set, before
// the shader's own: the size of the view the shader fills in points, and
// the pixels a point.
const (
	RectSizeUniform   = "__rectSize"
	ScaleUniform      = "__scale"
	ImageSizesUniform = "__imageSizes"
)

// MaxImages is how many images a shader reads at most: image0 to image3.
const MaxImages = 4

// BridgeLanes is how many lanes the bridge's uniforms of p take, before
// the shader's own: the view's size and scale, then each image's size in
// pixels.
func BridgeLanes(p *shaderir.Program) int { return 3 + 2*p.TextureCount }

// imagesOf returns how many images src reads: one more than the highest
// index of an image function it calls.
func imagesOf(src []byte) (int, error) {
	n := 0
	// Compiled here, not at package level, so apps that compile no shader
	// link no regular expressions for it.
	imageCall := regexp.MustCompile(`\bimage([0-9])(At|Size)\b`)
	for _, m := range imageCall.FindAllSubmatch(src, -1) {
		i, _ := strconv.Atoi(string(m[1]))
		if i >= MaxImages {
			return 0, fmt.Errorf("shader: image%d: a shader reads %d images at most", i, MaxImages)
		}
		n = max(n, i+1)
	}
	return n, nil
}

// imageBridge is the source of the image functions of n images, whose
// textures the platform binds in order.
func imageBridge(n int) string {
	if n == 0 {
		return ""
	}
	var b bytes.Buffer
	fmt.Fprintf(&b, "\nvar %s [%d]vec2\n", ImageSizesUniform, n)
	for i := range n {
		fmt.Fprintf(&b, `
// image%[1]dSize returns the size of image %[1]d in its pixels.
func image%[1]dSize() vec2 {
	return %[2]s[%[1]d]
}

// image%[1]dAt returns the color of image %[1]d at pos, in its pixels from
// its top left corner, premultiplied: transparent outside the image.
func image%[1]dAt(pos vec2) vec4 {
	in := step(vec2(0), pos) - step(%[2]s[%[1]d], pos)
	return __texelAt(__t%[1]d, pos) * in.x * in.y
}
`, i, ImageSizesUniform)
	}
	return b.String()
}

// bridgeSource is the source appended to a shader: the uniform variables
// that only platforms set, the functions that read them, and the vertex
// entry point, which only gives the fragment entry point its arguments:
// platforms draw with vertex stages of their own.
func bridgeSource() string {
	return fmt.Sprintf(`
%s

var %s vec2
var %s float

// rectSize returns the size of the view the shader fills, in points.
func rectSize() vec2 {
	return %[2]s
}

// pixelScale returns how many pixels of the view make a point.
func pixelScale() float {
	return %[3]s
}

func %s(dst vec2, pos vec2) (vec4, vec2) {
	return vec4(dst, 0, 1), pos
}
%s
`, InternalRegionBegin, RectSizeUniform, ScaleUniform, vertexEntry, InternalRegionEnd)
}

const vertexEntry = "__vertex"

// CompileFragment compiles an NSGo shader: Go's syntax in package main,
// with uniform variables (exported, package level) that the app sets and
// the entry point Fragment, which may call functions of its own.
func CompileFragment(src []byte) (*shaderir.Program, error) {
	images, err := imagesOf(src)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	b.Write(src)
	// One internal region holds the bridge and the image functions.
	bridge := bridgeSource()
	end := bytes.LastIndex([]byte(bridge), []byte(InternalRegionEnd))
	b.WriteString(bridge[:end])
	b.WriteString(imageBridge(images))
	b.WriteString(bridge[end:])
	p, err := Compile(b.Bytes(), vertexEntry, Entry, images)
	if err != nil {
		return nil, err
	}
	if p.FragmentFunc.Block == nil {
		return nil, fmt.Errorf("shader: no entry point %s", Entry)
	}
	if len(p.UniformNames) < 2 || p.UniformNames[0] != RectSizeUniform || p.UniformNames[1] != ScaleUniform ||
		images > 0 && (len(p.UniformNames) < 3 || p.UniformNames[2] != ImageSizesUniform) {
		return nil, fmt.Errorf("shader: the uniforms start with %v, not the bridge's", p.UniformNames)
	}
	p.FragmentSource = bytes.Clone(src)
	return p, nil
}
