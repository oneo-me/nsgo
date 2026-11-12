package apk

import (
	"encoding/binary"
	"testing"
	"unicode/utf16"

	androidruntime "github.com/nsgo-dev/nsgo/internal/platform/android/runtime"
)

// TestEncodeManifest reads back the binary XML the system parses, above
// all that each element's attributes ascend by framework ID: the system
// binary-searches them, and an out of order theme attribute is ignored.
func TestEncodeManifest(t *testing.T) {
	data, err := EncodeManifest(androidruntime.Manifest, Manifest{
		Package:     "dev.oneo.nsgo.test",
		Label:       "Test",
		VersionName: "1.2.3",
		VersionCode: 10203,
		Icon:        []byte("png"),
	})
	if err != nil {
		t.Fatal(err)
	}
	doc := decodeXML(t, data)

	root := doc.root
	if root.name != "manifest" {
		t.Fatalf("root = %q, want manifest", root.name)
	}
	if got := root.plain(t, "package"); got != "dev.oneo.nsgo.test" {
		t.Fatalf("package = %q", got)
	}
	if got := root.value(t, "versionCode"); got != 10203 {
		t.Fatalf("versionCode = %d", got)
	}
	if got := root.string(t, "versionName"); got != "1.2.3" {
		t.Fatalf("versionName = %q", got)
	}

	app := root.child(t, "application")
	if got := app.string(t, "label"); got != "Test" {
		t.Fatalf("label = %q", got)
	}
	if got, want := app.value(t, "theme"), uint32(0x01030129); got != want {
		t.Fatalf("theme = %#x, want %#x", got, want)
	}
	if got, want := app.value(t, "icon"), uint32(iconResourceID); got != want {
		t.Fatalf("icon = %#x, want %#x", got, want)
	}
	provider := app.child(t, "provider")
	if got := provider.string(t, "authorities"); got != "dev.oneo.nsgo.test.nsgo.files" {
		t.Fatalf("the file provider's authority = %q", got)
	}
	if got := provider.value(t, "grantUriPermissions"); got == 0 {
		t.Fatal("the file provider grants no URI permissions")
	}

	doc.walk(func(e *xmlNode) {
		last := uint32(0)
		for _, a := range e.attrs {
			if a.id == 0 {
				continue
			}
			if a.id < last {
				t.Fatalf("%s: %s (%#x) follows %#x", e.name, doc.strings[a.name], a.id, last)
			}
			last = a.id
		}
	})
	if len(doc.ids) > len(doc.strings) {
		t.Fatalf("resource map has %d IDs for %d strings", len(doc.ids), len(doc.strings))
	}
	for i, id := range doc.ids {
		if id == 0 {
			t.Fatalf("resource map entry %d is zero", i)
		}
		if i > 0 && id < doc.ids[i-1] {
			t.Fatalf("resource map does not ascend: %#x after %#x", id, doc.ids[i-1])
		}
	}
}

// TestEncodeManifestPermissions checks that the app's permissions join
// the runtime's once each, and that a name that is none fails.
func TestEncodeManifestPermissions(t *testing.T) {
	perms := []string{"android.permission.REQUEST_INSTALL_PACKAGES", "android.permission.INTERNET"}
	data, err := EncodeManifest(androidruntime.Manifest, Manifest{Package: "dev.oneo.nsgo.test", Permissions: perms})
	if err != nil {
		t.Fatal(err)
	}
	asked := map[string]int{}
	for _, kid := range decodeXML(t, data).root.kids {
		if kid.name == "uses-permission" {
			asked[kid.string(t, "name")]++
		}
	}
	for _, p := range perms {
		if asked[p] != 1 {
			t.Errorf("the manifest asks for %s %d times: %v", p, asked[p], asked)
		}
	}
	if _, err := EncodeManifest(androidruntime.Manifest, Manifest{Package: "dev.oneo.nsgo.test", Permissions: []string{"not a permission"}}); err == nil {
		t.Error("a bad permission was accepted")
	}
}

func TestEncodeManifestRejectsBadIDs(t *testing.T) {
	for _, id := range []string{"", "hello", "dev..nsgo", "dev.nsgo.hello world", "1dev.nsgo.hello"} {
		if _, err := EncodeManifest(androidruntime.Manifest, Manifest{Package: id}); err == nil {
			t.Errorf("id %q was accepted", id)
		}
	}
}

func TestVersionCode(t *testing.T) {
	for _, test := range []struct {
		version string
		want    int
	}{
		{"1.2.3", 10203},
		{"0.1.0", 100},
		{"0.1", 100},
		{"2", 20000},
		{"1.2.3.4", 10203},
		{"dev", 1},
		{"0.0.0", 1},
		{"1.2.-3", 1},
	} {
		if got := VersionCode(test.version); got != test.want {
			t.Errorf("VersionCode(%q) = %d, want %d", test.version, got, test.want)
		}
	}
}

// decoded is what the tests read out of the binary XML.

type xmlNode struct {
	name  string
	attrs []xmlAttribute
	kids  []*xmlNode
	doc   *xmlDocument
}

type xmlAttribute struct {
	ns   int32
	id   uint32 // the framework ID from the resource map
	name uint32
	kind uint8
	data uint32
}

type xmlDocument struct {
	strings []string
	ids     []uint32
	root    *xmlNode
}

// string returns the android attribute's string value.
func (n *xmlNode) string(t *testing.T, name string) string {
	t.Helper()
	return stringAt(t, n, n.attr(t, name).data)
}

// plain returns an attribute's string value, namespaced or not.
func (n *xmlNode) plain(t *testing.T, name string) string {
	t.Helper()
	for _, a := range n.attrs {
		if a.id == 0 && n.doc.strings[a.name] == name {
			return n.doc.strings[a.data]
		}
	}
	t.Fatalf("%s: no attribute %q", n.name, name)
	return ""
}

// value returns the android attribute's typed value.
func (n *xmlNode) value(t *testing.T, name string) uint32 {
	t.Helper()
	return n.attr(t, name).data
}

func (n *xmlNode) attr(t *testing.T, name string) xmlAttribute {
	t.Helper()
	for _, a := range n.attrs {
		if a.id == androidAttrs[name].id {
			return a
		}
	}
	t.Fatalf("%s: no android:%s", n.name, name)
	return xmlAttribute{}
}

func (n *xmlNode) child(t *testing.T, name string) *xmlNode {
	t.Helper()
	for _, kid := range n.kids {
		if kid.name == name {
			return kid
		}
	}
	t.Fatalf("%s: no %s child", n.name, name)
	return nil
}

func stringAt(t *testing.T, n *xmlNode, i uint32) string {
	t.Helper()
	if int(i) >= len(n.doc.strings) {
		t.Fatalf("%s: string %d of %d", n.name, i, len(n.doc.strings))
	}
	return n.doc.strings[i]
}

// decodeXML parses what EncodeManifest writes, with a reader of its own so
// the test does not trust the writer.
func decodeXML(t *testing.T, data []byte) xmlDocument {
	t.Helper()
	u16 := func(off int) uint16 { return binary.LittleEndian.Uint16(data[off:]) }
	u32 := func(off int) uint32 { return binary.LittleEndian.Uint32(data[off:]) }
	if got := u16(0); got != xmlHeader {
		t.Fatalf("header type = %#x", got)
	}
	if got := u32(4); got != uint32(len(data)) {
		t.Fatalf("file size = %d, want %d", got, len(data))
	}
	doc := xmlDocument{}
	var stack []*xmlNode
	off := 0
	// The header is a chunk like any other; skip its own fields.
	off = 8
	for off < len(data) {
		kind, size := u16(off), int(u32(off+4))
		if size < 8 || off+size > len(data) {
			t.Fatalf("chunk %#x at %d has size %d", kind, off, size)
		}
		switch kind {
		case stringPool:
			if u32(off+16)&0x100 != 0 {
				t.Fatal("string pool is not the UTF-16 the writer makes")
			}
			count := int(u32(off + 8))
			start := off + int(u32(off+20))
			for i := 0; i < count; i++ {
				p := start + int(u32(off+28+4*i))
				n := int(u16(p))
				p += 2
				if n&0x8000 != 0 {
					n = n&0x7fff<<16 | int(u16(p))
					p += 2
				}
				units := make([]uint16, n)
				for j := range units {
					units[j] = u16(p + 2*j)
				}
				doc.strings = append(doc.strings, string(utf16.Decode(units)))
			}
		case resourceMap:
			for p := 8; p+4 <= size; p += 4 {
				doc.ids = append(doc.ids, u32(off+p))
			}
		case xmlStartElement:
			n := &xmlNode{name: doc.strings[u32(off+20)], doc: &doc}
			count := int(u16(off + 28))
			for i := 0; i < count; i++ {
				a := off + 36 + 20*i
				attr := xmlAttribute{
					ns:   int32(u32(a)),
					name: u32(a + 4),
					kind: data[a+15],
					data: u32(a + 16),
				}
				if attr.ns >= 0 && int(attr.name) < len(doc.ids) {
					attr.id = doc.ids[attr.name]
				}
				n.attrs = append(n.attrs, attr)
			}
			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				parent.kids = append(parent.kids, n)
			} else if doc.root == nil {
				doc.root = n
			}
			stack = append(stack, n)
		case xmlEndElement:
			stack = stack[:len(stack)-1]
		}
		off += size
	}
	if doc.root == nil {
		t.Fatal("no root element")
	}
	return doc
}

// walk visits the nodes of the tree in order.
func (d xmlDocument) walk(f func(*xmlNode)) {
	var visit func(*xmlNode)
	visit = func(n *xmlNode) {
		f(n)
		for _, kid := range n.kids {
			visit(kid)
		}
	}
	visit(d.root)
}
