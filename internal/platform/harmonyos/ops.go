//go:build js && wasm

package harmonyos

import (
	"github.com/nsgo-dev/nsgo/internal/driver"
	batch "github.com/nsgo-dev/nsgo/internal/platform/ops"
)

// ops writes a batch's operations as JSON, which the shared writer builds:
// this backend keeps the names its mount uses, as every backend does.
type ops struct{ batch.Writer }

func (o *ops) reset()               { o.Writer.Reset() }
func (o *ops) empty() bool          { return o.Writer.Empty() }
func (o *ops) bytes() []byte        { return o.Writer.Bytes() }
func (o *ops) num(v float64) *ops   { o.Writer.Num(v); return o }
func (o *ops) str(s string) *ops    { o.Writer.Str(s); return o }
func (o *ops) boolean(v bool) *ops  { o.Writer.Bool(v); return o }
func (o *ops) null() *ops           { o.Writer.Null(); return o }
func (o *ops) raw(json []byte) *ops { o.Writer.Raw(json); return o }

func (o *ops) op(code string, id driver.NodeID) *ops {
	o.Writer.Op(code, id)
	return o
}
