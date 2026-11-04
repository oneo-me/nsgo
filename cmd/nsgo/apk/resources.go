package apk

import (
	"bytes"
	"encoding/binary"
	"slices"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// The resource table chunk types, from Android's ResourceTypes.h.
const (
	tableHeader     = 0x0002
	tablePackage    = 0x0200
	tableType       = 0x0201
	tableTypeSpec   = 0x0202
	appPackageID    = 0x7f
	configSize      = 64
	densityXXXHigh  = 640
	iconFile        = "res/mipmap-xxxhdpi-v4/ic_launcher.png"
	iconResource    = "@mipmap/ic_launcher"
	iconResourceID  = appPackageID<<24 | 1<<16 // package 0x7f, type 1, entry 0
	nameResource    = "@string/app_name"
	utf8PoolFlag    = 1 << 8
	resStringHeader = 28

	// What a type's entry varies by (ResTable_config's CONFIG_*).
	configLocale  = 0x0004
	configDensity = 0x0100
)

// IconSize is the launcher icon's size in pixels: 48dp at xxxhdpi, the
// density the table declares it for. The system scales it for others.
const IconSize = 192

// nameResourceID is @string/app_name's ID: the type after the icon's,
// when the table has one.
func nameResourceID(icon bool) uint32 {
	if icon {
		return appPackageID<<24 | 2<<16
	}
	return appPackageID<<24 | 1<<16
}

// resType is a type of the table with one entry, key, and its value in
// each configuration.
type resType struct {
	name, key string
	varies    uint32 // configLocale, configDensity
	values    []resValue
}

// resValue is a string value of an entry in a configuration.
type resValue struct {
	config []byte
	str    string
}

// resources is the resource table an APK carries: the launcher icon
// @mipmap/ic_launcher when icon, at xxxhdpi in iconFile, and the app's
// name @string/app_name in each language of names, by BCP 47 tag, ""
// for the default, when names has any. The manifest refers to them by
// iconResourceID and nameResourceID.
func resources(pkg string, icon bool, names map[string]string) []byte {
	var types []resType
	if icon {
		config := newConfig()
		binary.LittleEndian.PutUint16(config[14:], densityXXXHigh)
		binary.LittleEndian.PutUint16(config[24:], 4) // sdkVersion, as aapt2 adds for a density
		types = append(types, resType{"mipmap", "ic_launcher", configDensity, []resValue{{config, iconFile}}})
	}
	if len(names) > 0 {
		t := resType{name: "string", key: "app_name", varies: configLocale}
		langs := make([]string, 0, len(names))
		for lang := range names {
			if lang == "" || localeConfig(lang) != nil {
				langs = append(langs, lang)
			}
		}
		slices.Sort(langs)
		for _, lang := range langs {
			config := newConfig()
			if lang != "" {
				config = localeConfig(lang)
			}
			t.values = append(t.values, resValue{config, names[lang]})
		}
		types = append(types, t)
	}
	return table(pkg, types)
}

func newConfig() []byte {
	config := make([]byte, configSize)
	binary.LittleEndian.PutUint32(config, configSize)
	return config
}

// localeConfig is the configuration of a BCP 47 tag's language, script
// and region, as aapt2 writes a b+ qualifier; nil for a tag it cannot
// hold.
func localeConfig(tag string) []byte {
	parts := strings.Split(tag, "-")
	lang, ok := packCode(parts[0], 'a')
	if !ok {
		return nil
	}
	config := newConfig()
	copy(config[8:10], lang[:])
	for _, p := range parts[1:] {
		switch {
		case len(p) == 4:
			copy(config[36:40], p)
		case len(p) == 2 || len(p) == 3 && p[0] >= '0' && p[0] <= '9':
			region, ok := packCode(p, '0')
			if !ok {
				return nil
			}
			copy(config[10:12], region[:])
		default:
			return nil
		}
	}
	return config
}

// packCode packs a language or region of two or three characters as
// ResTable_config does: two as they are, three in 15 bits by their
// distance from base ('a' for languages, '0' for regions).
func packCode(s string, base byte) ([2]byte, bool) {
	switch len(s) {
	case 2:
		return [2]byte{s[0], s[1]}, true
	case 3:
		a, b, c := s[0]-base, s[1]-base, s[2]-base
		if a > 0x1f || b > 0x1f || c > 0x1f {
			return [2]byte{}, false
		}
		return [2]byte{0x80 | c<<2 | b>>3, b<<5 | a}, true
	}
	return [2]byte{}, false
}

// table writes a resource table of the app's package with types, each
// type numbered from 1 with one entry.
func table(pkg string, types []resType) []byte {
	put := func(b *bytes.Buffer, v any) { binary.Write(b, binary.LittleEndian, v) }
	var values, typeNames, keys []string
	var chunks bytes.Buffer
	for i, t := range types {
		id := uint8(i + 1)
		typeNames = append(typeNames, t.name)
		keys = append(keys, t.key)
		put(&chunks, [2]uint16{tableTypeSpec, 16})
		put(&chunks, uint32(16+4))
		put(&chunks, [4]uint8{id, 0, 0, 0})
		put(&chunks, uint32(1)) // one entry
		put(&chunks, t.varies)
		for _, v := range t.values {
			const typeHeader = 20 + configSize
			put(&chunks, [2]uint16{tableType, typeHeader})
			put(&chunks, uint32(typeHeader+4+16))
			put(&chunks, [4]uint8{id, 0, 0, 0})
			put(&chunks, uint32(1))            // entries
			put(&chunks, uint32(typeHeader+4)) // where they start
			chunks.Write(v.config)
			put(&chunks, uint32(0))               // the entry's offset
			put(&chunks, [2]uint16{8, 0})         // entry header, no flags
			put(&chunks, uint32(i))               // its key
			put(&chunks, uint16(8))               // value size
			put(&chunks, [2]uint8{0, typeString}) // a string,
			put(&chunks, uint32(len(values)))     // in the global pool
			values = append(values, v.str)
		}
	}

	typePool, keyPool := stringPoolChunk(typeNames), stringPoolChunk(keys)
	const packageHeader = 288
	var p bytes.Buffer
	put(&p, [2]uint16{tablePackage, packageHeader})
	put(&p, uint32(packageHeader+len(typePool)+len(keyPool)+chunks.Len()))
	put(&p, uint32(appPackageID))
	name := make([]uint16, 128)
	copy(name[:127], utf16.Encode([]rune(pkg)))
	put(&p, name)
	put(&p, uint32(packageHeader))               // type strings
	put(&p, uint32(len(types)))                  // last public type
	put(&p, uint32(packageHeader+len(typePool))) // key strings
	put(&p, uint32(len(keys)))                   // last public key
	put(&p, uint32(0))                           // type ID offset
	p.Write(typePool)
	p.Write(keyPool)
	p.Write(chunks.Bytes())

	var body bytes.Buffer
	body.Write(stringPoolChunk(values))
	if len(types) > 0 {
		body.Write(p.Bytes())
	}
	var out bytes.Buffer
	put(&out, [2]uint16{tableHeader, 12})
	put(&out, uint32(12+body.Len()))
	packages := uint32(0)
	if len(types) > 0 {
		packages = 1
	}
	put(&out, packages)
	out.Write(body.Bytes())
	return out.Bytes()
}

// stringPoolChunk is a UTF-8 string pool, each string with its length in
// UTF-16 units and in bytes and a terminating zero.
func stringPoolChunk(strs []string) []byte {
	var data bytes.Buffer
	length := func(n int) {
		if n > 0x7f {
			data.WriteByte(byte(0x80 | n>>8))
		}
		data.WriteByte(byte(n))
	}
	offsets := make([]uint32, len(strs))
	for i, s := range strs {
		offsets[i] = uint32(data.Len())
		if !utf8.ValidString(s) {
			s = strings.ToValidUTF8(s, string(utf8.RuneError))
		}
		units := 0
		for _, r := range s {
			units += utf16.RuneLen(r)
		}
		length(units)
		length(len(s))
		data.WriteString(s)
		data.WriteByte(0)
	}
	for data.Len()%4 != 0 {
		data.WriteByte(0)
	}
	start := resStringHeader + 4*len(strs)
	var out bytes.Buffer
	put := func(v any) { binary.Write(&out, binary.LittleEndian, v) }
	put([2]uint16{stringPool, resStringHeader})
	put(uint32(start + data.Len()))
	put(uint32(len(strs)))
	put(uint32(0)) // styles
	put(uint32(utf8PoolFlag))
	put(uint32(start))
	put(uint32(0)) // styles start
	put(offsets)
	out.Write(data.Bytes())
	return out.Bytes()
}
