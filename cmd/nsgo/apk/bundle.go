package apk

import (
	"encoding/binary"
	"fmt"
	"io"
	"slices"
	"strings"
)

// An Android App Bundle is what Google Play takes: a ZIP of the base
// module, whose manifest and resource table are protocol buffers, as
// aapt2's Resources.proto and Configuration.proto define them, rather
// than binary XML, and BundleConfig.pb, bundletool's config.proto, from
// which Play makes each device's APKs. The bundle is signed as a JAR, with
// the upload key.

// BundletoolVersion is the bundletool the bundle says it is made for.
const BundletoolVersion = "1.18.3"

// WriteBundle writes a signed Android App Bundle to w: template's manifest
// with m filled in, files as the base module's, by their paths in an APK,
// and the resource table, which holds the launcher icon and the app's
// names when m has them.
func WriteBundle(w io.Writer, template []byte, m Manifest, files []File, key *Key) error {
	root, err := appManifest(template, m)
	if err != nil {
		return err
	}
	manifest, err := protoElement(root, true, m.Icon != nil)
	if err != nil {
		return err
	}
	all := []File{{Name: "base/manifest/AndroidManifest.xml", Data: pbBytes(nil, 1, manifest)}}
	for _, f := range files {
		name, ok := bundlePath(f.Name)
		if !ok {
			return fmt.Errorf("bundle: %s has no place in a module", f.Name)
		}
		all = append(all, File{Name: name, Data: f.Data, Stored: f.Stored})
	}
	if m.Icon != nil {
		all = append(all, File{Name: "base/" + iconFile, Data: m.Icon, Stored: true})
	}
	all = append(all,
		File{Name: "base/resources.pb", Data: resourcesProto(m.Package, m.Icon != nil, m.Names)},
		File{Name: "BundleConfig.pb", Data: bundleConfig()})
	signed, err := signJar(all, key)
	if err != nil {
		return err
	}
	zip, err := archive(signed)
	if err != nil {
		return err
	}
	_, err = w.Write(zip)
	return err
}

// bundlePath is where a file of an APK goes in the base module: the dex
// files under dex/, the rest, such as lib/ and assets/, as they are.
func bundlePath(name string) (string, bool) {
	switch {
	case strings.HasSuffix(name, ".dex") && !strings.Contains(name, "/"):
		return "base/dex/" + name, true
	case strings.HasPrefix(name, "lib/"), strings.HasPrefix(name, "assets/"), strings.HasPrefix(name, "res/"):
		return "base/" + name, true
	}
	return "", false
}

// bundleConfig is BundleConfig.pb: the bundletool version, and native
// libraries kept compressed, so the system extracts them, as the
// runtime starts the app's executable from the libraries' directory.
func bundleConfig() []byte {
	tool := pbString(nil, 2, BundletoolVersion)
	// Optimizations.uncompress_native_libraries, present with enabled
	// false.
	optimizations := pbBytes(nil, 2, nil)
	out := pbBytes(nil, 1, tool)
	return pbBytes(out, 2, optimizations)
}

// protoElement encodes n as an XmlNode's element, with the android
// namespace declared on the root, and the values the attributes compile
// to as the binary manifest does.
func protoElement(n *node, root, icon bool) ([]byte, error) {
	var el []byte
	if root {
		ns := pbString(nil, 1, "android")
		ns = pbString(ns, 2, androidNS)
		el = pbBytes(el, 1, ns)
	}
	el = pbString(el, 3, n.name.Local)
	for _, a := range n.attrs {
		if a.Name.Local == "xmlns" || a.Name.Space == "xmlns" {
			continue
		}
		var attr []byte
		switch a.Name.Space {
		case "":
			attr = pbString(attr, 2, a.Name.Local)
			attr = pbString(attr, 3, a.Value)
		case androidNS:
			spec, ok := androidAttrs[a.Name.Local]
			if !ok {
				return nil, fmt.Errorf("manifest: packaging does not know android:%s", a.Name.Local)
			}
			attr = pbString(attr, 1, androidNS)
			attr = pbString(attr, 2, a.Name.Local)
			attr = pbString(attr, 3, a.Value)
			attr = pbVarint(attr, 5, uint64(spec.id))
			item, err := protoItem(a.Name.Local, a.Value, spec, icon)
			if err != nil {
				return nil, err
			}
			if item != nil {
				attr = pbBytes(attr, 6, item)
			}
		default:
			return nil, fmt.Errorf("manifest: unknown namespace %q", a.Name.Space)
		}
		el = pbBytes(el, 4, attr)
	}
	for _, kid := range n.kids {
		child, err := protoElement(kid, false, icon)
		if err != nil {
			return nil, err
		}
		el = pbBytes(el, 5, pbBytes(nil, 1, child))
	}
	return el, nil
}

// protoItem is the Item an android:* attribute compiles to; nil for a
// string, which stays its value alone.
func protoItem(name, value string, spec attrSpec, icon bool) ([]byte, error) {
	if name == "label" && value == nameResource {
		return pbBytes(nil, 1, pbVarint(nil, 2, uint64(nameResourceID(icon)))), nil
	}
	if spec.kind == typeString {
		return nil, nil
	}
	v, err := spec.parse(value)
	if err != nil {
		return nil, fmt.Errorf("manifest: android:%s: %w", name, err)
	}
	var prim []byte
	switch spec.kind {
	case typeReference:
		return pbBytes(nil, 1, pbVarint(nil, 2, uint64(v))), nil
	case typeInt:
		prim = pbVarint(nil, 6, uint64(int64(int32(v))))
	case typeHex:
		prim = pbVarint(nil, 7, uint64(v))
	case typeBool:
		b := uint64(0)
		if v != 0 {
			b = 1
		}
		prim = pbVarint(nil, 8, b)
	}
	return pbBytes(nil, 7, prim), nil
}

// resourcesProto is the module's resource table, resources.pb, holding
// what resources' binary table does.
func resourcesProto(pkg string, icon bool, names map[string]string) []byte {
	var types [][]byte
	id := uint64(1)
	if icon {
		config := pbVarint(nil, 18, densityXXXHigh)
		config = pbVarint(config, 24, 4)
		file := pbString(nil, 1, iconFile)
		file = pbVarint(file, 2, 1) // PNG
		types = append(types, protoType(id, "mipmap", "ic_launcher", [][2][]byte{{config, pbBytes(nil, 5, file)}}))
		id++
	}
	if len(names) > 0 {
		var langs []string
		for lang := range names {
			if lang == "" || localeConfig(lang) != nil {
				langs = append(langs, lang)
			}
		}
		slices.Sort(langs)
		var values [][2][]byte
		for _, lang := range langs {
			var config []byte
			if lang != "" {
				config = pbString(nil, 3, lang)
			}
			values = append(values, [2][]byte{config, pbBytes(nil, 2, pbString(nil, 1, names[lang]))})
		}
		types = append(types, protoType(id, "string", "app_name", values))
	}
	p := pbBytes(nil, 1, pbVarint(nil, 1, appPackageID))
	p = pbString(p, 2, pkg)
	for _, t := range types {
		p = pbBytes(p, 3, t)
	}
	return pbBytes(nil, 2, p)
}

// protoType is a Type of one entry, key, with an item in each of its
// configurations.
func protoType(id uint64, name, key string, values [][2][]byte) []byte {
	entry := pbBytes(nil, 1, nil) // entry 0
	entry = pbString(entry, 2, key)
	for _, v := range values {
		cv := pbBytes(nil, 1, v[0])
		cv = pbBytes(cv, 2, pbBytes(nil, 4, v[1]))
		entry = pbBytes(entry, 6, cv)
	}
	t := pbBytes(nil, 1, pbVarint(nil, 1, id))
	t = pbString(t, 2, name)
	return pbBytes(t, 3, entry)
}

// The protocol buffer wire format: each field a key, its number and wire
// type, then a varint or a length and bytes.

func pbKey(b []byte, field, wire int) []byte {
	return binary.AppendUvarint(b, uint64(field)<<3|uint64(wire))
}

func pbVarint(b []byte, field int, v uint64) []byte {
	return binary.AppendUvarint(pbKey(b, field, 0), v)
}

func pbBytes(b []byte, field int, v []byte) []byte {
	b = binary.AppendUvarint(pbKey(b, field, 2), uint64(len(v)))
	return append(b, v...)
}

func pbString(b []byte, field int, s string) []byte {
	return pbBytes(b, field, []byte(s))
}
