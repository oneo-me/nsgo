package terminal

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"fmt"
	"image"

	"github.com/oneo-me/nsgo/base/terminal/internal/vt"
)

// imageKey is a placement of an image as a Mirror placed it in the other
// terminal: row counts from the top of the scrollback, so that it stays
// as the screen scrolls.
type imageKey struct {
	img, id                    uint32
	row, col, xoff, yoff       int
	cols, rows                 int
	srcX, srcY, srcW, srcH, gr int
	z                          int32
	virtual                    bool
}

// mirroredImages are the images a Mirror sent of a screen, by id, at
// their generations, and the placements it made, with their ids in the
// other terminal.
type mirroredImages struct {
	gens   map[uint32]uint64
	placed map[imageKey]uint32
}

// images brings the other terminal's images of the screen that shows to
// this one's (Kitty graphics): it sends the pixels of those placed, once
// for each generation, places what shows that it has not, and deletes
// what went. A placement whose top scrolled off before a frame saw it, as
// when a Mirror starts over, does not show there. top is the screen's
// first row from the top of the scrollback.
func (m *Mirror) images(out *bytes.Buffer, term *vt.Terminal, alt bool, top, rows int) {
	st := &m.imgs[0]
	if alt {
		st = &m.imgs[1]
	}
	if st.gens == nil {
		st.gens, st.placed = map[uint32]uint64{}, map[imageKey]uint32{}
	}
	seen := map[imageKey]bool{}
	var place []imageKey
	term.Placements(func(pl vt.Placement) {
		if pl.Virtual {
			// Placed by its id, which the placeholder cells name.
			k := imageKey{img: pl.ImageID, id: pl.ID, cols: pl.ParamCols, rows: pl.ParamRows, z: pl.Z, virtual: true}
			seen[k] = true
			if _, ok := st.placed[k]; !ok {
				place = append(place, k)
			}
			return
		}
		if !pl.Visible {
			return
		}
		k := imageKey{
			img: pl.ImageID, id: pl.ID, row: top + pl.Row, col: pl.Col, xoff: pl.XOffset, yoff: pl.YOffset,
			cols: pl.ParamCols, rows: pl.ParamRows,
			srcX: pl.ParamSrcX, srcY: pl.ParamSrcY, srcW: pl.ParamSrcW, srcH: pl.ParamSrcH, gr: max(pl.Rows, 1), z: pl.Z,
		}
		seen[k] = true
		if _, ok := st.placed[k]; !ok && pl.Row >= 0 {
			place = append(place, k)
		}
	})
	// Images gone, or of new pixels, which the other terminal's placements
	// of them may not survive: placed again where they show.
	for id, gen := range st.gens {
		g := term.ImageGeneration(id)
		if g == gen {
			continue
		}
		if g == 0 {
			fmt.Fprintf(out, "\x1b_Ga=d,d=I,i=%d,q=2\x1b\\", id)
			delete(st.gens, id)
		}
		for k := range st.placed {
			if k.img == id {
				delete(st.placed, k)
				if g != 0 && seen[k] && (k.virtual || k.row >= top) {
					place = append(place, k)
				}
			}
		}
	}
	// Placements gone from where they would show; those that scrolled off
	// stay, as they do here.
	for k, id := range st.placed {
		if !seen[k] && (k.virtual || k.row+k.gr > top && k.row < top+rows) {
			fmt.Fprintf(out, "\x1b_Ga=d,d=i,i=%d,p=%d,q=2\x1b\\", k.img, id)
			delete(st.placed, k)
		}
	}
	for _, k := range place {
		gen := term.ImageGeneration(k.img)
		if gen == 0 {
			continue
		}
		if st.gens[k.img] != gen {
			img, ok := term.Image(k.img)
			if !ok {
				continue
			}
			sendImage(out, img)
			st.gens[k.img] = gen
		}
		if k.virtual {
			fmt.Fprintf(out, "\x1b_Ga=p,U=1,i=%d,p=%d,c=%d,r=%d,z=%d,q=2\x1b\\", k.img, k.id, k.cols, k.rows, k.z)
			st.placed[k] = k.id
			continue
		}
		m.placement++
		fmt.Fprintf(out, "\x1b[%d;%dH\x1b_Ga=p,i=%d,p=%d,X=%d,Y=%d,x=%d,y=%d,w=%d,h=%d,c=%d,r=%d,z=%d,C=1,q=2\x1b\\",
			k.row-top+1, k.col+1, k.img, m.placement, k.xoff, k.yoff, k.srcX, k.srcY, k.srcW, k.srcH, k.cols, k.rows, k.z)
		st.placed[k] = m.placement
	}
}

// deleteImages deletes from the other terminal the images sent of a
// screen, with their placements, wherever they are.
func (m *Mirror) deleteImages(out *bytes.Buffer, st *mirroredImages) {
	for id := range st.gens {
		fmt.Fprintf(out, "\x1b_Ga=d,d=I,i=%d,q=2\x1b\\", id)
	}
}

// sendImage writes the sequences that transmit an image's pixels, as
// RGBA compressed (o=z), in chunks.
func sendImage(out *bytes.Buffer, img vt.Image) {
	var z bytes.Buffer
	w := zlib.NewWriter(&z)
	w.Write(decoded(img).Pix)
	w.Close()
	data := base64.StdEncoding.EncodeToString(z.Bytes())
	const chunk = 4096
	for i := 0; i == 0 || i < len(data); i += chunk {
		part := data[i:min(i+chunk, len(data))]
		more := 0
		if i+chunk < len(data) {
			more = 1
		}
		if i == 0 {
			fmt.Fprintf(out, "\x1b_Ga=t,f=32,o=z,s=%d,v=%d,i=%d,q=2,m=%d;%s\x1b\\", img.Width, img.Height, img.ID, more, part)
		} else {
			fmt.Fprintf(out, "\x1b_Gm=%d;%s\x1b\\", more, part)
		}
	}
}

// decoded returns an image's pixels as an image.
func decoded(img vt.Image) *image.NRGBA {
	w, h := img.Width, img.Height
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	px := img.Pixels
	switch img.Format {
	case vt.ImageRGBA:
		copy(out.Pix, px)
	case vt.ImageRGB:
		for i := 0; i < w*h && 3*i+2 < len(px); i++ {
			copy(out.Pix[4*i:], px[3*i:3*i+3])
			out.Pix[4*i+3] = 0xff
		}
	case vt.ImageGrayAlpha:
		for i := 0; i < w*h && 2*i+1 < len(px); i++ {
			g := px[2*i]
			out.Pix[4*i], out.Pix[4*i+1], out.Pix[4*i+2], out.Pix[4*i+3] = g, g, g, px[2*i+1]
		}
	case vt.ImageGray:
		for i := 0; i < w*h && i < len(px); i++ {
			g := px[i]
			out.Pix[4*i], out.Pix[4*i+1], out.Pix[4*i+2], out.Pix[4*i+3] = g, g, g, 0xff
		}
	}
	return out
}
