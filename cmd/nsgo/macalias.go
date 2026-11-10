package main

import (
	"encoding/binary"
	"path/filepath"
	"strings"
	"unicode/utf16"
)

// A disk image's background picture is named in its .DS_Store by a Mac
// alias, version 2, the record Finder resolves files from: the volume's
// name and creation date, the folder's and the file's catalog node IDs
// (the inodes of HFS+), their names and the file's path on the volume,
// as the mac_alias Python package writes for dmgbuild. Its fields come
// from the mounted image, whose IDs and dates the compressed copy keeps.

// macAliasInfo is what an alias records of a file.
type macAliasInfo struct {
	volume        string // the volume's name
	volumeCreated uint32 // in seconds since 1904
	parentCNID    uint32
	parentName    string
	fileCNID      uint32
	fileCreated   uint32
	fileName      string
	path          string // from the volume's root, "/.background/x.tiff"
}

// macEpoch is 1904-01-01 in seconds since 1970, from which Mac dates count.
const macEpoch = 2082844800

// macAlias encodes an alias of a file on a disk image's volume.
func macAlias(a macAliasInfo) []byte {
	be := binary.BigEndian
	pstr := func(s string, size int) []byte {
		b := make([]byte, size+1)
		s = macRoman(s)
		if len(s) > size {
			s = s[:size]
		}
		b[0] = byte(len(s))
		copy(b[1:], s)
		return b
	}
	var b []byte
	b = append(b, 0, 0, 0, 0) // application
	b = be.AppendUint16(b, 0) // the record's size, below
	b = be.AppendUint16(b, 2) // version
	b = be.AppendUint16(b, 0) // a file
	b = append(b, pstr(a.volume, 27)...)
	b = be.AppendUint32(b, a.volumeCreated)
	b = append(b, "H+"...)
	b = be.AppendUint16(b, 0) // a fixed disk, as macOS reports images
	b = be.AppendUint32(b, a.parentCNID)
	b = append(b, pstr(a.fileName, 63)...)
	b = be.AppendUint32(b, a.fileCNID)
	b = be.AppendUint32(b, a.fileCreated)
	b = append(b, 0, 0, 0, 0, 0, 0, 0, 0) // type and creator
	b = be.AppendUint16(b, 0xffff)        // levels from the root: unknown
	b = be.AppendUint16(b, 0xffff)        // levels to the target: unknown
	b = be.AppendUint32(b, 0)             // volume attributes
	b = be.AppendUint16(b, 0)             // volume file system ID
	b = append(b, make([]byte, 10)...)    // reserved
	extra := func(tag uint16, data []byte) {
		b = be.AppendUint16(b, tag)
		b = be.AppendUint16(b, uint16(len(data)))
		b = append(b, data...)
		if len(data)%2 == 1 {
			b = append(b, 0)
		}
	}
	unicode := func(s string) []byte {
		u := utf16.Encode([]rune(s))
		d := be.AppendUint16(nil, uint16(len(u)))
		for _, c := range u {
			d = be.AppendUint16(d, c)
		}
		return d
	}
	// High-resolution dates count 1/65536 seconds.
	date := func(seconds uint32) []byte { return be.AppendUint64(nil, uint64(seconds)<<16) }
	extra(0, []byte(macRoman(a.parentName)))
	extra(16, date(a.volumeCreated))
	extra(17, date(a.fileCreated))
	extra(1, be.AppendUint32(nil, a.parentCNID))
	// The Carbon path, the volume's name and the path in colons.
	extra(2, []byte(macRoman(a.volume+strings.ReplaceAll(a.path, "/", ":"))))
	extra(14, unicode(a.fileName))
	extra(15, unicode(a.volume))
	extra(18, []byte(a.path))
	extra(19, []byte(filepath.Join("/Volumes", a.volume)))
	b = be.AppendUint16(b, 0xffff) // the end
	b = be.AppendUint16(b, 0)
	be.PutUint16(b[4:], uint16(len(b)))
	return b
}

// macRoman keeps s's ASCII, which Mac Roman shares, and replaces the rest:
// the Unicode fields hold the names whole.
func macRoman(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x80 {
			return r
		}
		return '?'
	}, s)
}
