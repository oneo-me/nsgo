//go:build darwin

package apple

import (
	"strings"
	"sync"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/driver"
)

var utiOnce sync.Once

// families are the uniform types of the media type families a pick
// names (image/*), the others offering every file.
var families = map[string]string{
	"image": "public.image", "video": "public.movie", "audio": "public.audio", "text": "public.text",
	"font": "public.font", "model": "public.3d-content",
}

// UTType is the UTType class, its framework loaded.
func UTType() objc.ID {
	utiOnce.Do(func() {
		purego.Dlopen("/System/Library/Frameworks/UniformTypeIdentifiers.framework/UniformTypeIdentifiers", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	})
	return objc.ID(Class("UTType"))
}

// ContentTypes returns the UTTypes of the files r offers, for
// NSOpenPanel and UIDocumentPickerViewController: every file, as
// public.data, when r names none or one the system has no type for.
func ContentTypes(r driver.FilesRequest) objc.ID {
	class := UTType()
	types := objc.ID(Class("NSMutableArray")).Send(sel("array"))
	byExtension := func(ext string) objc.ID {
		return class.Send(sel("typeWithFilenameExtension:"), NSString(ext))
	}
	every := len(r.Types) == 0
	for _, t := range r.Types {
		var found []objc.ID
		if ext, ok := strings.CutPrefix(t, "."); ok {
			found = append(found, byExtension(ext))
		} else if family, ok := strings.CutSuffix(t, "/*"); ok {
			if id := families[family]; id != "" {
				found = append(found, class.Send(sel("typeWithIdentifier:"), NSString(id)))
			}
		} else if ut := class.Send(sel("typeWithMIMEType:"), NSString(t)); ut != 0 && !objc.Send[bool](ut, sel("isDynamic")) {
			found = append(found, ut)
		} else {
			// A type the system does not know: its files' extensions.
			exts, _ := driver.FilesRequest{Types: []string{t}}.Extensions()
			for _, ext := range exts {
				found = append(found, byExtension(ext))
			}
		}
		if len(found) == 0 || found[0] == 0 {
			every = true
			break
		}
		for _, ut := range found {
			types.Send(sel("addObject:"), ut)
		}
	}
	if every {
		types = objc.ID(Class("NSMutableArray")).Send(sel("array"))
		types.Send(sel("addObject:"), class.Send(sel("typeWithIdentifier:"), NSString("public.data")))
	}
	return types
}

// SaveType returns the UTType of a file named name, for a save dialog
// to keep its extension, or 0 for a name without one.
func SaveType(name string) objc.ID {
	i := strings.LastIndexByte(name, '.')
	if i <= 0 || i == len(name)-1 {
		return 0
	}
	return UTType().Send(sel("typeWithFilenameExtension:"), NSString(name[i+1:]))
}
