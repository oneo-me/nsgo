//go:build darwin

package apple

import (
	"slices"
	"testing"

	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/driver"
)

// identifiers returns the identifiers of an array of UTTypes.
func identifiers(types objc.ID) []string {
	var ids []string
	ForEach(types, func(t objc.ID) { ids = append(ids, GoString(t.Send(sel("identifier")))) })
	return ids
}

func TestContentTypesNameTheSystemsTypes(t *testing.T) {
	for _, c := range []struct {
		types []string
		want  []string
	}{
		{[]string{".zip", "application/pdf", "image/*"}, []string{"public.zip-archive", "com.adobe.pdf", "public.image"}},
		{[]string{"application/x-nsgo-unknown"}, []string{"public.data"}},
		{[]string{"application/*"}, []string{"public.data"}},
		{nil, []string{"public.data"}},
	} {
		if got := identifiers(ContentTypes(driver.FilesRequest{Types: c.types})); !slices.Equal(got, c.want) {
			t.Errorf("%v offers %v, want %v", c.types, got, c.want)
		}
	}
	// APKs, whose identifier differs between releases, are not every file.
	if got := identifiers(ContentTypes(driver.FilesRequest{Types: []string{"application/vnd.android.package-archive"}})); len(got) != 1 || got[0] == "public.data" {
		t.Errorf("APKs offer %v", got)
	}
	if got := GoString(SaveType("Notes.txt").Send(sel("identifier"))); got != "public.plain-text" {
		t.Errorf("a text file saves as %s", got)
	}
	if SaveType("Makefile") != 0 {
		t.Error("a name without an extension has a type")
	}
}
