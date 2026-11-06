// Package woff unpacks WOFF and WOFF2 font files into the TrueType or
// OpenType data they hold, which every platform's text engine registers.
// Import turns them around as a TSX bundle is built, so it links only
// where the bundler does.
package woff

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"slices"
)

// maxTables is the most a file's tables may add up to, as NSH's
// font_data: a few bytes of a forged file could otherwise ask for
// gigabytes.
const maxTables = 128 << 20

// Is reports whether data is a WOFF or WOFF2 file.
func Is(data []byte) bool {
	return len(data) >= 4 && (string(data[:4]) == "wOFF" || string(data[:4]) == "wOF2")
}

// Unpack returns the TrueType or OpenType data of a font file: data
// itself unless it is a WOFF or WOFF2 file.
func Unpack(data []byte) ([]byte, error) {
	if !Is(data) {
		return data, nil
	}
	if string(data[:4]) == "wOFF" {
		return unpack1(data)
	}
	return unpack2(data)
}

// table is a table of the font being rebuilt.
type table struct {
	tag  uint32
	data []byte
}

// unpack1 is WOFF: a directory of tables each deflated on its own.
func unpack1(data []byte) ([]byte, error) {
	const header, entry = 44, 20
	if len(data) < header {
		return nil, errors.New("woff: the header is cut short")
	}
	be := binary.BigEndian
	n := int(be.Uint16(data[12:]))
	if header+n*entry > len(data) {
		return nil, errors.New("woff: the table directory is cut short")
	}
	tables := make([]table, 0, n)
	total := 0
	for i := range n {
		e := data[header+i*entry:]
		offset, compressed, length := int64(be.Uint32(e[4:])), int64(be.Uint32(e[8:])), int64(be.Uint32(e[12:]))
		if compressed > length {
			return nil, fmt.Errorf("woff: table %d is larger compressed (%d bytes) than it is (%d)", i, compressed, length)
		}
		if total += int(length); total > maxTables {
			return nil, fmt.Errorf("woff: the tables declare more than a font may hold (%d bytes)", maxTables)
		}
		if offset+compressed > int64(len(data)) {
			return nil, fmt.Errorf("woff: table %d is cut short", i)
		}
		raw := data[offset : offset+compressed]
		if compressed < length {
			z, err := zlib.NewReader(bytes.NewReader(raw))
			if err != nil {
				return nil, fmt.Errorf("woff: table %d: %w", i, err)
			}
			raw = make([]byte, length)
			if _, err := io.ReadFull(z, raw); err != nil {
				return nil, fmt.Errorf("woff: table %d: %w", i, err)
			}
		}
		tables = append(tables, table{be.Uint32(e), raw})
	}
	return assemble(be.Uint32(data[4:]), tables)
}

// assemble lays tables out as a font file: its directory in tag order,
// each table on four bytes and the checksums, head's over the whole file
// the last, counted again, since a rebuilt table's changed.
func assemble(flavor uint32, tables []table) ([]byte, error) {
	const header, entry = 12, 16
	slices.SortFunc(tables, func(a, b table) int {
		if a.tag < b.tag {
			return -1
		}
		if a.tag > b.tag {
			return 1
		}
		return 0
	})
	size := header + len(tables)*entry
	for _, t := range tables {
		size += (len(t.data) + 3) &^ 3
	}
	out := make([]byte, size)
	be := binary.BigEndian
	be.PutUint32(out, flavor)
	n := len(tables)
	be.PutUint16(out[4:], uint16(n))
	log, power := 0, 1
	for power*2 <= n {
		log, power = log+1, power*2
	}
	be.PutUint16(out[6:], uint16(power*16))
	be.PutUint16(out[8:], uint16(log))
	be.PutUint16(out[10:], uint16((n-power)*16))
	at, head := header+n*entry, -1
	for i, t := range tables {
		if i > 0 && t.tag == tables[i-1].tag {
			return nil, fmt.Errorf("woff: two %q tables", tagName(t.tag))
		}
		if t.tag == tagHead {
			if len(t.data) < 12 {
				return nil, errors.New("woff: head is cut short")
			}
			head = at
		}
		copy(out[at:], t.data)
		if t.tag == tagHead {
			// Summed with its adjustment zero, as the format says.
			clear(out[at+8 : at+12])
		}
		e := out[header+i*entry:]
		be.PutUint32(e, t.tag)
		be.PutUint32(e[4:], checksum(out[at:at+len(t.data)]))
		be.PutUint32(e[8:], uint32(at))
		be.PutUint32(e[12:], uint32(len(t.data)))
		at += (len(t.data) + 3) &^ 3
	}
	if head >= 0 {
		be.PutUint32(out[head+8:], 0xB1B0AFBA-checksum(out))
	}
	return out, nil
}

// checksum sums data as big-endian words, the last padded with zeros.
func checksum(data []byte) uint32 {
	var sum uint32
	for ; len(data) >= 4; data = data[4:] {
		sum += binary.BigEndian.Uint32(data)
	}
	var last [4]byte
	copy(last[:], data)
	return sum + binary.BigEndian.Uint32(last[:])
}

func tagName(tag uint32) string {
	return string([]byte{byte(tag >> 24), byte(tag >> 16), byte(tag >> 8), byte(tag)})
}

func tagOf(s string) uint32 { return binary.BigEndian.Uint32([]byte(s)) }

var (
	tagGlyf = tagOf("glyf")
	tagLoca = tagOf("loca")
	tagHead = tagOf("head")
	tagHhea = tagOf("hhea")
	tagHmtx = tagOf("hmtx")
	tagMaxp = tagOf("maxp")
)
