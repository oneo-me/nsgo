package main

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"slices"
	"strings"
)

// A SquashFS 4.0 image, as an AppImage carries after its runtime, written
// in Go where no Mac has mksquashfs: gzip's zlib streams compress its
// 128 KB data blocks and 8 KB metadata blocks, every file's tail is a
// block of its own rather than a fragment, and it has no extended
// attributes or export table. Everything is owned by root.

const (
	sqfsMagic     = 0x73717368
	sqfsBlockSize = 128 << 10
	sqfsBlockLog  = 17
	sqfsMetaSize  = 8 << 10
	sqfsGzip      = 1
	// The superblock's flags: no fragments and no extended attributes.
	sqfsNoFragments = 0x0010
	sqfsNoXattrs    = 0x0200
	sqfsNone        = ^uint64(0)

	// Inode types; directory entries name the basic ones.
	sqfsDir     = 1
	sqfsFile    = 2
	sqfsSymlink = 3
	sqfsLDir    = 8

	// A block size's bit for a block stored uncompressed.
	sqfsUncompressedBlock = 1 << 24
	// A metadata block header's bit for a block stored uncompressed.
	sqfsUncompressedMeta = 0x8000
)

// sqfsNode is a file, directory or link of the image.
type sqfsNode struct {
	name string
	mode uint16
	data []byte
	link string
	dir  bool
	kids []*sqfsNode

	inode uint32 // its inode's number
	ref   uint64 // where its inode is: its block's start << 16 | offset
}

// sqfsTree builds the image's tree from entries named by their paths,
// creating the directories they are in.
func sqfsTree(entries []linuxPackageEntry) (*sqfsNode, error) {
	root := &sqfsNode{dir: true, mode: 0o755}
	dirs := map[string]*sqfsNode{"": root}
	var dirOf func(name string) *sqfsNode
	dirOf = func(name string) *sqfsNode {
		if d, ok := dirs[name]; ok {
			return d
		}
		parent, base := "", name
		if i := strings.LastIndexByte(name, '/'); i >= 0 {
			parent, base = name[:i], name[i+1:]
		}
		d := &sqfsNode{name: base, dir: true, mode: 0o755}
		p := dirOf(parent)
		p.kids = append(p.kids, d)
		dirs[name] = d
		return d
	}
	for _, e := range entries {
		if e.dir {
			dirOf(e.name)
			continue
		}
		parent, base := "", e.name
		if i := strings.LastIndexByte(e.name, '/'); i >= 0 {
			parent, base = e.name[:i], e.name[i+1:]
		}
		if base == "" {
			return nil, errors.New("squashfs: a file has no name")
		}
		p := dirOf(parent)
		p.kids = append(p.kids, &sqfsNode{name: base, mode: uint16(e.mode), data: e.data, link: e.link})
	}
	return root, nil
}

// sqfsMeta writes a metadata table: a stream cut into 8 KB blocks, each
// compressed unless that makes it no smaller.
type sqfsMeta struct {
	out     bytes.Buffer // the blocks written
	pending bytes.Buffer // the block being filled
}

// pos is where the next bytes go: the start of the block being filled,
// relative to the table, and the offset in it.
func (m *sqfsMeta) pos() (uint32, uint16) {
	return uint32(m.out.Len()), uint16(m.pending.Len())
}

func (m *sqfsMeta) write(b []byte) {
	for len(b) > 0 {
		n := min(len(b), sqfsMetaSize-m.pending.Len())
		m.pending.Write(b[:n])
		b = b[n:]
		if m.pending.Len() == sqfsMetaSize {
			m.flush()
		}
	}
}

func (m *sqfsMeta) flush() {
	if m.pending.Len() == 0 {
		return
	}
	data := m.pending.Bytes()
	if z := zlibBytes(data); len(z) < len(data) {
		binary.Write(&m.out, binary.LittleEndian, uint16(len(z)))
		m.out.Write(z)
	} else {
		binary.Write(&m.out, binary.LittleEndian, uint16(len(data))|sqfsUncompressedMeta)
		m.out.Write(data)
	}
	m.pending.Reset()
}

func zlibBytes(data []byte) []byte {
	var buf bytes.Buffer
	w, _ := zlib.NewWriterLevel(&buf, zlib.BestCompression)
	w.Write(data)
	w.Close()
	return buf.Bytes()
}

// squashfs is the image of root's tree, modified at mtime.
func squashfs(root *sqfsNode, mtime uint32) ([]byte, error) {
	var img bytes.Buffer
	img.Write(make([]byte, 96)) // the superblock, written last
	// Number the inodes, children before their directory, as they are
	// written; the root's is the last.
	var count uint32
	var number func(n *sqfsNode)
	number = func(n *sqfsNode) {
		slices.SortFunc(n.kids, func(a, b *sqfsNode) int { return strings.Compare(a.name, b.name) })
		for _, k := range n.kids {
			number(k)
		}
		count++
		n.inode = count
	}
	number(root)

	// The files' data, block by block.
	type fileData struct {
		start uint64
		sizes []uint32
	}
	files := map[*sqfsNode]fileData{}
	var walkData func(n *sqfsNode)
	walkData = func(n *sqfsNode) {
		for _, k := range n.kids {
			walkData(k)
		}
		if n.dir || n.link != "" || len(n.data) == 0 {
			return
		}
		fd := fileData{start: uint64(img.Len())}
		for data := n.data; len(data) > 0; {
			block := data[:min(len(data), sqfsBlockSize)]
			data = data[len(block):]
			if z := zlibBytes(block); len(z) < len(block) {
				img.Write(z)
				fd.sizes = append(fd.sizes, uint32(len(z)))
			} else {
				img.Write(block)
				fd.sizes = append(fd.sizes, uint32(len(block))|sqfsUncompressedBlock)
			}
		}
		files[n] = fd
	}
	walkData(root)
	if img.Len() > 0xffffffff {
		return nil, errors.New("squashfs: the files take more than 4 GB")
	}

	var inodes, dirs sqfsMeta
	le := binary.LittleEndian
	header := func(b []byte, kind uint16, n *sqfsNode) []byte {
		b = le.AppendUint16(b, kind)
		b = le.AppendUint16(b, n.mode&0o7777)
		b = le.AppendUint16(b, 0) // uid's index
		b = le.AppendUint16(b, 0) // gid's index
		b = le.AppendUint32(b, mtime)
		return le.AppendUint32(b, n.inode)
	}
	var write func(n, parent *sqfsNode) error
	write = func(n, parent *sqfsNode) error {
		for _, k := range n.kids {
			if err := write(k, n); err != nil {
				return err
			}
		}
		var b []byte
		switch {
		case n.dir:
			// The listing first, in runs of entries whose inodes share a
			// metadata block and are near in number.
			start, offset := dirs.pos()
			size := 0
			for i := 0; i < len(n.kids); {
				first := n.kids[i]
				block := uint32(first.ref >> 16)
				j := i
				for j < len(n.kids) && j-i < 256 && uint32(n.kids[j].ref>>16) == block &&
					int64(n.kids[j].inode)-int64(first.inode) >= -32768 && int64(n.kids[j].inode)-int64(first.inode) <= 32767 {
					j++
				}
				var run []byte
				run = le.AppendUint32(run, uint32(j-i-1))
				run = le.AppendUint32(run, block)
				run = le.AppendUint32(run, first.inode)
				for _, k := range n.kids[i:j] {
					kind := uint16(sqfsFile)
					switch {
					case k.dir:
						kind = sqfsDir
					case k.link != "":
						kind = sqfsSymlink
					}
					if len(k.name) > 256 {
						return errors.New("squashfs: a name is longer than 256 bytes")
					}
					run = le.AppendUint16(run, uint16(k.ref&0xffff))
					run = le.AppendUint16(run, uint16(int16(int64(k.inode)-int64(first.inode))))
					run = le.AppendUint16(run, kind)
					run = le.AppendUint16(run, uint16(len(k.name)-1))
					run = append(run, k.name...)
				}
				dirs.write(run)
				size += len(run)
				i = j
			}
			links := uint32(2)
			for _, k := range n.kids {
				if k.dir {
					links++
				}
			}
			parentInode := count + 1
			if parent != nil {
				parentInode = parent.inode
			}
			// The size counts the . and .. entries a listing once had.
			if size+3 <= 0xffff {
				b = header(b, sqfsDir, n)
				b = le.AppendUint32(b, start)
				b = le.AppendUint32(b, links)
				b = le.AppendUint16(b, uint16(size+3))
				b = le.AppendUint16(b, offset)
				b = le.AppendUint32(b, parentInode)
			} else {
				b = header(b, sqfsLDir, n)
				b = le.AppendUint32(b, links)
				b = le.AppendUint32(b, uint32(size+3))
				b = le.AppendUint32(b, start)
				b = le.AppendUint32(b, parentInode)
				b = le.AppendUint16(b, 0) // no index
				b = le.AppendUint16(b, offset)
				b = le.AppendUint32(b, 0xffffffff) // no extended attributes
			}
		case n.link != "":
			b = header(b, sqfsSymlink, n)
			b = le.AppendUint32(b, 1)
			b = le.AppendUint32(b, uint32(len(n.link)))
			b = append(b, n.link...)
		default:
			fd := files[n]
			b = header(b, sqfsFile, n)
			b = le.AppendUint32(b, uint32(fd.start))
			b = le.AppendUint32(b, 0xffffffff) // no fragment
			b = le.AppendUint32(b, 0)
			b = le.AppendUint32(b, uint32(len(n.data)))
			for _, s := range fd.sizes {
				b = le.AppendUint32(b, s)
			}
		}
		block, offset := inodes.pos()
		n.ref = uint64(block)<<16 | uint64(offset)
		inodes.write(b)
		return nil
	}
	if err := write(root, nil); err != nil {
		return nil, err
	}
	inodes.flush()
	dirs.flush()

	inodeStart := uint64(img.Len())
	img.Write(inodes.out.Bytes())
	dirStart := uint64(img.Len())
	img.Write(dirs.out.Bytes())
	// The ID table: one metadata block holding root's ID, 0, and its
	// index, which points at it.
	var ids sqfsMeta
	ids.write(le.AppendUint32(nil, 0))
	ids.flush()
	idBlock := uint64(img.Len())
	img.Write(ids.out.Bytes())
	idStart := uint64(img.Len())
	img.Write(le.AppendUint64(nil, idBlock))
	used := uint64(img.Len())

	sb := make([]byte, 0, 96)
	sb = le.AppendUint32(sb, sqfsMagic)
	sb = le.AppendUint32(sb, count)
	sb = le.AppendUint32(sb, mtime)
	sb = le.AppendUint32(sb, sqfsBlockSize)
	sb = le.AppendUint32(sb, 0) // fragments
	sb = le.AppendUint16(sb, sqfsGzip)
	sb = le.AppendUint16(sb, sqfsBlockLog)
	sb = le.AppendUint16(sb, sqfsNoFragments|sqfsNoXattrs)
	sb = le.AppendUint16(sb, 1) // IDs
	sb = le.AppendUint16(sb, 4)
	sb = le.AppendUint16(sb, 0)
	sb = le.AppendUint64(sb, root.ref)
	sb = le.AppendUint64(sb, used)
	sb = le.AppendUint64(sb, idStart)
	sb = le.AppendUint64(sb, sqfsNone) // extended attributes
	sb = le.AppendUint64(sb, inodeStart)
	sb = le.AppendUint64(sb, dirStart)
	sb = le.AppendUint64(sb, idBlock)  // the fragment table, empty
	sb = le.AppendUint64(sb, sqfsNone) // export table
	out := img.Bytes()
	copy(out, sb)
	// Images are padded to 4 KB, as mksquashfs pads them for devices.
	if pad := len(out) % 4096; pad != 0 {
		out = append(out, make([]byte, 4096-pad)...)
	}
	return out, nil
}
