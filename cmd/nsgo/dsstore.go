package main

// This file comes from MyGo, https://github.com/egoist/mygo, Copyright (c)
// 2026 MyGo contributors, licensed under the MIT License (NOTICE): it
// writes the .DS_Store file that lays out the Finder window of a disk
// image, so building the image needs no AppleScript or Finder automation
// permission and works headless. The layout matches the ds_store Python
// package dmgbuild uses: the output is byte for byte what
// DSStore.open(path, "w+") writes for the same records.

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"slices"
	"strings"
	"unicode/utf16"
)

// dsRecord is one .DS_Store record: a property (code) of a file in the
// folder, or of the folder itself when name is ".".
type dsRecord struct {
	name  string
	code  string // four characters
	kind  string // "long", "blob" or "type"
	value []byte // big-endian number, blob bytes or four-character type
}

func dsLong(name, code string, v uint32) dsRecord {
	return dsRecord{name, code, "long", binary.BigEndian.AppendUint32(nil, v)}
}

func dsBlob(name, code string, b []byte) dsRecord { return dsRecord{name, code, "blob", b} }

// dsIconLocation places the center of a file's icon, in points from the
// top-left corner of the window content.
func dsIconLocation(name string, x, y int) dsRecord {
	b := binary.BigEndian.AppendUint32(nil, uint32(x))
	b = binary.BigEndian.AppendUint32(b, uint32(y))
	b = append(b, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0, 0)
	return dsBlob(name, "Iloc", b)
}

func (r dsRecord) encode(b []byte) []byte {
	name := utf16.Encode([]rune(r.name))
	b = binary.BigEndian.AppendUint32(b, uint32(len(name)))
	for _, u := range name {
		b = binary.BigEndian.AppendUint16(b, u)
	}
	b = append(b, r.code...)
	b = append(b, r.kind...)
	if r.kind == "blob" {
		b = binary.BigEndian.AppendUint32(b, uint32(len(r.value)))
	}
	return append(b, r.value...)
}

// dsStore encodes a .DS_Store file holding records.
//
// The file is a buddy-allocated address space (after a 4-byte prefix) with a
// header at 0, the B-tree header ("DSDB") at 0x20, the allocator's root block
// at 0x800 and a single B-tree leaf at 0x2000. The records must fit in that
// leaf, which is plenty for the few records of a disk image window.
func dsStore(records []dsRecord) ([]byte, error) {
	records = slices.Clone(records)
	slices.SortStableFunc(records, func(a, b dsRecord) int {
		if c := strings.Compare(strings.ToLower(a.name), strings.ToLower(b.name)); c != 0 {
			return c
		}
		return strings.Compare(a.code, b.code)
	})
	node := binary.BigEndian.AppendUint32(nil, 0) // a leaf: no child pointer
	node = binary.BigEndian.AppendUint32(node, uint32(len(records)))
	for _, r := range records {
		node = r.encode(node)
	}
	const pageSize = 0x1000
	if len(node) > pageSize {
		return nil, fmt.Errorf(".DS_Store records need %d bytes, more than one page", len(node))
	}

	const (
		dsdbAddr = 0x20   // block 1, 2^5 bytes
		rootAddr = 0x800  // block 0, 2^11 bytes
		nodeAddr = 0x2000 // block 2, 2^13 bytes
	)
	file := make([]byte, 4+0x4000)
	file[3] = 1
	mem := file[4:] // allocator addresses are relative to this
	be := binary.BigEndian

	copy(mem, "Bud1")
	be.PutUint32(mem[4:], rootAddr)
	be.PutUint32(mem[8:], 0x800) // root block size
	be.PutUint32(mem[12:], rootAddr)
	copy(mem[16:], []byte{0, 0, 0x10, 0x0c, 0, 0, 0, 0x87, 0, 0, 0x20, 0x0b, 0, 0, 0, 0})

	be.PutUint32(mem[dsdbAddr:], 2) // root node: block 2
	be.PutUint32(mem[dsdbAddr+4:], 0)
	be.PutUint32(mem[dsdbAddr+8:], uint32(len(records)))
	be.PutUint32(mem[dsdbAddr+12:], 1) // nodes
	be.PutUint32(mem[dsdbAddr+16:], pageSize)

	root := mem[rootAddr:]
	be.PutUint32(root, 3) // blocks
	for i, addr := range []uint32{rootAddr | 11, dsdbAddr | 5, nodeAddr | 13} {
		be.PutUint32(root[8+4*i:], addr) // an offset with its log2 size
	}
	p := 8 + 256*4 // the block list is padded to 256 entries
	be.PutUint32(root[p:], 1)
	root[p+4] = 4
	copy(root[p+5:], "DSDB")
	be.PutUint32(root[p+9:], 1) // the B-tree header is block 1
	p += 13
	// Free lists by log2 size. Carving the blocks above out of the initial
	// 2 GiB leaves one free block of each other size at an offset equal to
	// its size.
	for width := range 32 {
		if width < 6 || width == 11 || width == 13 || width == 31 {
			p += 4
			continue
		}
		be.PutUint32(root[p:], 1)
		be.PutUint32(root[p+4:], 1<<width)
		p += 8
	}

	copy(mem[nodeAddr:], node)
	return file, nil
}

// binaryPlist encodes a flat dictionary of bools, ints, float64s and strings
// as a binary property list, laid out like Python's plistlib does it.
func binaryPlist(dict map[string]any) []byte {
	keys := make([]string, 0, len(dict))
	for k := range dict {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	// Objects: the dictionary, then its keys and values with equal scalars
	// shared.
	type scalar struct {
		kind  string
		value any
	}
	objs := []any{dict}
	refs := map[scalar]int{}
	ref := func(v any) int {
		k := scalar{fmt.Sprintf("%T", v), v}
		if b, ok := v.([]byte); ok {
			// Slices are no map keys: data is shared by its bytes.
			k.value = string(b)
		}
		if i, ok := refs[k]; ok {
			return i
		}
		refs[k] = len(objs)
		objs = append(objs, v)
		return len(objs) - 1
	}
	keyRefs := make([]int, len(keys))
	for i, k := range keys {
		keyRefs[i] = ref(k)
	}
	valRefs := make([]int, len(keys))
	for i, k := range keys {
		valRefs[i] = ref(dict[k])
	}

	sizeOf := func(n int) int {
		switch v := uint64(n); {
		case v < 1<<8:
			return 1
		case v < 1<<16:
			return 2
		case v < 1<<32:
			return 4
		}
		return 8
	}
	putUint := func(b []byte, v uint64, size int) []byte {
		for i := size - 1; i >= 0; i-- {
			b = append(b, byte(v>>(8*i)))
		}
		return b
	}
	refSize := sizeOf(len(objs))
	appendInt := func(b []byte, v int) []byte {
		switch {
		case v < 0:
			return putUint(append(b, 0x13), uint64(v), 8)
		case v < 1<<8:
			return append(b, 0x10, byte(v))
		case v < 1<<16:
			return putUint(append(b, 0x11), uint64(v), 2)
		case int64(v) < 1<<32:
			return putUint(append(b, 0x12), uint64(v), 4)
		}
		return putUint(append(b, 0x13), uint64(v), 8)
	}
	appendSize := func(b []byte, marker byte, n int) []byte {
		if n < 15 {
			return append(b, marker|byte(n))
		}
		return appendInt(append(b, marker|0xf), n)
	}

	var buf bytes.Buffer
	buf.WriteString("bplist00")
	offsets := make([]int, len(objs))
	for i, o := range objs {
		offsets[i] = buf.Len()
		var b []byte
		switch v := o.(type) {
		case map[string]any:
			b = appendSize(b, 0xd0, len(keys))
			for _, r := range keyRefs {
				b = putUint(b, uint64(r), refSize)
			}
			for _, r := range valRefs {
				b = putUint(b, uint64(r), refSize)
			}
		case bool:
			b = []byte{0x08}
			if v {
				b[0] = 0x09
			}
		case int:
			b = appendInt(b, v)
		case float64:
			b = putUint([]byte{0x23}, math.Float64bits(v), 8)
		case []byte:
			b = append(appendSize(b, 0x40, len(v)), v...)
		case string:
			if isASCII(v) {
				b = append(appendSize(b, 0x50, len(v)), v...)
			} else {
				u := utf16.Encode([]rune(v))
				b = appendSize(b, 0x60, len(u))
				for _, c := range u {
					b = binary.BigEndian.AppendUint16(b, c)
				}
			}
		default:
			panic(fmt.Sprintf("binaryPlist: unsupported %T", o))
		}
		buf.Write(b)
	}
	tableOffset := buf.Len()
	offsetSize := sizeOf(tableOffset)
	var t []byte
	for _, off := range offsets {
		t = putUint(t, uint64(off), offsetSize)
	}
	t = append(t, 0, 0, 0, 0, 0, 0, byte(offsetSize), byte(refSize))
	t = putUint(t, uint64(len(objs)), 8)
	t = putUint(t, 0, 8) // the top object
	t = putUint(t, uint64(tableOffset), 8)
	buf.Write(t)
	return buf.Bytes()
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

// dmgLayout is the Finder window of a disk image: its size in points, the
// centers of the app's and the Applications link's icons, and the alias
// of its background picture on the volume, if it has one.
type dmgLayout struct {
	Width, Height   int
	AppX, AppY      int
	ApplicationsX   int
	ApplicationsY   int
	IconSize        int
	BackgroundAlias []byte
}

// dmgWindowX and dmgWindowY place the window's bottom-left corner, from the
// bottom of the screen. The window's bounds hold its title bar,
// dmgTitleBar points high on macOS 26 and later, over the picture.
const (
	dmgWindowX, dmgWindowY = 200, 400
	dmgTitleBar            = 32
)

// dmgHidden are the volume's hidden files, which Finder shows when the
// user asks for hidden files: the layout puts them below the window.
var dmgHidden = []string{".background", ".DS_Store", ".fseventsd", ".Trashes", ".VolumeIcon.icns"}

// dmgLabelDrop is how far below an icon's center the middle of the icon
// and its name together falls, in points: a 128-point icon's artwork
// reaches 51 points above its center, and the name below it 92 points
// below, as Finder draws them with 16-point labels.
const dmgLabelDrop = 20

// layoutFor places the app and the Applications link a quarter in from
// either side of a window of w×h points, each icon with its name centered
// from top to bottom, where the built-in background's arrow runs between
// them.
func layoutFor(w, h int) dmgLayout {
	y := h/2 - dmgLabelDrop
	return dmgLayout{Width: w, Height: h, AppX: w * 27 / 100, AppY: y, ApplicationsX: w * 73 / 100, ApplicationsY: y, IconSize: 128}
}

// dmgRecords lays out the window of a disk image holding app (the bundle's
// file name) and an Applications link.
func dmgRecords(app string, l dmgLayout) []dsRecord {
	bwsp := map[string]any{
		"ShowStatusBar":         false,
		"WindowBounds":          fmt.Sprintf("{{%d, %d}, {%d, %d}}", dmgWindowX, dmgWindowY, l.Width, l.Height+dmgTitleBar),
		"ContainerShowSidebar":  false,
		"PreviewPaneVisibility": false,
		"SidebarWidth":          180,
		"ShowTabView":           false,
		"ShowToolbar":           false,
		"ShowPathbar":           false,
		"ShowSidebar":           false,
	}
	icvp := map[string]any{
		"viewOptionsVersion":   1,
		"backgroundType":       0,
		"backgroundColorRed":   1.0,
		"backgroundColorGreen": 1.0,
		"backgroundColorBlue":  1.0,
		"gridOffsetX":          0.0,
		"gridOffsetY":          0.0,
		"gridSpacing":          100.0,
		"arrangeBy":            "none",
		"showIconPreview":      false,
		"showItemInfo":         false,
		"labelOnBottom":        true,
		"textSize":             16.0,
		"iconSize":             float64(l.IconSize),
		"scrollPositionX":      0.0,
		"scrollPositionY":      0.0,
	}
	if l.BackgroundAlias != nil {
		// A picture, which Finder finds by its alias.
		icvp["backgroundType"] = 2
		icvp["backgroundImageAlias"] = l.BackgroundAlias
	}
	records := []dsRecord{
		dsLong(".", "vSrn", 1),
		dsBlob(".", "bwsp", binaryPlist(bwsp)),
		dsBlob(".", "icvp", binaryPlist(icvp)),
		{".", "icvl", "type", []byte("icnv")}, // icon view
		dsIconLocation(app, l.AppX, l.AppY),
		dsIconLocation("Applications", l.ApplicationsX, l.ApplicationsY),
	}
	for i, name := range dmgHidden {
		records = append(records, dsIconLocation(name, 100+i*150, l.Height+200))
	}
	return records
}

// writeDSStore writes the layout of a disk image window to path.
func writeDSStore(path, app string, l dmgLayout) error {
	data, err := dsStore(dmgRecords(app, l))
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
