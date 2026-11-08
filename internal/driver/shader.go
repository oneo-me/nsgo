package driver

import "github.com/oneo-me/nsgo/internal/shaderir"

// ShaderProgram is a fragment shader an app compiled: the program the
// core checked, so platforms translate it for their GPU without linking
// the compiler. Each translates it once and keeps the result while the
// program is in use. Once handed to the platform it never changes, so
// platforms key what they made from it by its pointer.
type ShaderProgram struct {
	IR *shaderir.Program
	// WGSL is set for a program written in WGSL (package shader/wgsl):
	// its first lanes are the built-ins (ShaderBuiltinLanes) and its
	// textures the pictures (Pictures), not images.
	WGSL bool
}

// ShaderBuiltinLanes is how many lanes a WGSL program's built-ins take
// before its uniforms: the time, the progress and the overflow.
const ShaderBuiltinLanes = 3

// Shader is what a shader view draws (ItemShader): Program with the
// values of its uniforms.
type Shader struct {
	Program *ShaderProgram
	// Lanes are the program's own uniforms, after those the platform
	// sets, lane by lane in the order they are declared: a vector's
	// components, a matrix's columns and an array's elements in turn.
	// An int, uint or bool lane takes the value's integer part.
	Lanes []float64
	// Nodes are the lanes animated nodes drive over Lanes, which the
	// platform reads from its Animator each frame they change.
	Nodes []ShaderNode
	// Images are what the program's image0At to image3At read, in order.
	// The platform loads each as an image view does and draws the view
	// again once it has them; one it lacks reads as transparent.
	Images []ImageSource

	// Role is what the shader draws: a shader view's fill, or over a
	// view's children, the surface or a change of pages. A WGSL program
	// reads the pictures Pictures names, which the platform takes of
	// the views the role says.
	Role     ShaderRole
	Pictures Pictures
	// Overflow is how far beyond each side of the view the shader draws,
	// and its pictures reach, in points.
	Overflow float64
	// Running is whether a transition runs: the shader draws over the
	// children from the picture taken as it started (ItemShaderTransition)
	// to what they show, hiding them, until a Shader that is not Running
	// ends it. Backwards takes the first picture as the side the change
	// goes to, sample_to, as a pop or a dismissal does.
	Running, Backwards bool
}

// ShaderNode binds lane Lane of a shader's uniforms to animated node
// Node: its number, or one channel of the color it outputs.
type ShaderNode struct {
	Lane    int
	Node    uint32
	Channel Channel
}

// Channel is what a shader lane reads of a node: its number, or its
// color's red, green, blue or alpha, from 0 to 1.
type Channel uint8

const (
	ChannelNumber Channel = iota
	ChannelRed
	ChannelGreen
	ChannelBlue
	ChannelAlpha
)

// Of is what c reads of a node's number and color.
func (c Channel) Of(num float64, color Color) float64 {
	if c == ChannelNumber {
		return num
	}
	r, g, b, a := color.Components()
	return [...]float64{0, r, g, b, a}[c]
}

// ShaderRole is what a shader draws.
type ShaderRole uint8

const (
	// ShaderFill fills a shader view's box under its children.
	ShaderFill ShaderRole = iota
	// ShaderEffect draws the view's children through the shader, over
	// them and in their place; they still take input.
	ShaderEffect
	// ShaderTransition draws the view's children as they change, while
	// Running, and shows them as they are between changes.
	ShaderTransition
	// ShaderScreen and ShaderScreenTransition do the same with the whole
	// surface, which a view of no size in it holds.
	ShaderScreen
	ShaderScreenTransition
	// ShaderNav draws a stack's change of pages, or the surface as a
	// dialog comes or goes (on the surface's root view), while Running:
	// sample_from the lower side and sample_to the upper.
	ShaderNav
)

// Covers reports whether r draws a whole surface.
func (r ShaderRole) Covers() bool { return r == ShaderScreen || r == ShaderScreenTransition }

// Transitions reports whether r draws a change while Running.
func (r ShaderRole) Transitions() bool {
	return r == ShaderTransition || r == ShaderScreenTransition || r == ShaderNav
}

// Pictures is a set of pictures a WGSL program reads, its textures in
// this order.
type Pictures uint8

const (
	// PictureContent is the children of an effect, or the surface for a
	// screen shader.
	PictureContent Pictures = 1 << iota
	// PictureBackdrop is what the surface shows behind the view.
	PictureBackdrop
	// PictureFrom and PictureTo are a transition's sides.
	PictureFrom
	PictureTo
)
