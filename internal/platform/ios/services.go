//go:build ios

package ios

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/apple"
)

// The services UIKit performs: the pasteboard's images and files,
// opening URLs, the photo picker, the document picker for files and
// saves, and the app's appearance.

func generalPasteboard() objc.ID {
	return objc.ID(objc.GetClass("UIPasteboard")).Send(sel("generalPasteboard"))
}

// The pasteboard types of images, in the order a read prefers them.
var imageTypes = []string{"public.png", "public.jpeg", "public.heic", "public.tiff", "com.compuserve.gif"}

// setClipboardImage puts the image data encodes on the pasteboard as a
// PNG.
func setClipboardImage(data []byte) {
	png, _, err := apple.PNG(data)
	if err != nil {
		return
	}
	generalPasteboard().Send(sel("setData:forPasteboardType:"), apple.NSData(png), apple.NSString("public.png"))
}

var uiImagePNG func(image objc.ID) objc.ID

// readClipboard answers an ItemReadClipboard. Asking whether the
// pasteboard holds text or images reads nothing, so iOS shows no paste
// notice for it.
func (s *surface) readClipboard(it *driver.MountItem) {
	pb := generalPasteboard()
	ev := driver.Event{Kind: driver.EventClipboard, Count: it.Count}
	switch it.Clipboard {
	case driver.ClipboardReadText:
		ev.Text = apple.GoString(pb.Send(sel("string")))
	case driver.ClipboardReadHasText:
		ev.Finished = objc.Send[bool](pb, sel("hasStrings"))
	case driver.ClipboardReadHasImage:
		ev.Finished = objc.Send[bool](pb, sel("hasImages"))
	case driver.ClipboardReadFiles:
		apple.ForEach(pb.Send(sel("URLs")), func(u objc.ID) {
			if objc.Send[bool](u, sel("isFileURL")) {
				ev.URIs = append(ev.URIs, apple.GoString(u.Send(sel("absoluteString"))))
			}
		})
	case driver.ClipboardReadImage:
		if !objc.Send[bool](pb, sel("hasImages")) {
			break
		}
		var data []byte
		for _, t := range imageTypes {
			if d := pb.Send(sel("dataForPasteboardType:"), apple.NSString(t)); d != 0 {
				data = apple.Bytes(d)
				break
			}
		}
		if data == nil {
			if uiImagePNG == nil {
				lib, err := purego.Dlopen(uikitPath, purego.RTLD_NOW)
				if err == nil {
					purego.RegisterLibFunc(&uiImagePNG, lib, "UIImagePNGRepresentation")
				}
			}
			if img := pb.Send(sel("image")); img != 0 && uiImagePNG != nil {
				data = apple.Bytes(uiImagePNG(img))
			}
		}
		id := it.Count
		go func() {
			ev := driver.Event{Kind: driver.EventClipboard, Count: id}
			if png, size, err := apple.PNG(data); err == nil && len(png) <= driver.MaxClipboardImage {
				ev.Data = &driver.ClipboardImage{PNG: png, Size: size}
			}
			s.engine.Dispatch(ev)
		}()
		return
	}
	s.engine.Dispatch(ev)
}

// openURL opens u in the app that handles it and reports whether one
// did.
func (s *surface) openURL(u string, id uint32) {
	nsURL := objc.ID(objc.GetClass("NSURL")).Send(sel("URLWithString:"), apple.NSString(u))
	if nsURL == 0 {
		s.engine.Dispatch(driver.Event{Kind: driver.EventOpenURL, Count: id})
		return
	}
	done := objc.NewBlock(func(_ objc.Block, ok bool) {
		s.engine.Dispatch(driver.Event{Kind: driver.EventOpenURL, Count: id, Finished: ok})
	})
	app := objc.ID(objc.GetClass("UIApplication")).Send(sel("sharedApplication"))
	opts := objc.ID(objc.GetClass("NSDictionary")).Send(sel("dictionary"))
	app.Send(sel("openURL:options:completionHandler:"), nsURL, opts, done)
	done.Release()
}

var (
	pickerOnce     sync.Once
	classPicker    objc.Class
	pickerRequests = map[objc.ID]pickRequest{}
)

type pickRequest struct {
	r  driver.PickRequest
	id uint32
}

// pickImages presents PHPickerViewController for the images r asks for;
// picking needs no permission. The picked images load and are prepared
// off the main thread, in the order picked.
func (s *surface) pickImages(r driver.PickRequest, id uint32) {
	pickerOnce.Do(func() {
		if _, err := purego.Dlopen("/System/Library/Frameworks/PhotosUI.framework/PhotosUI", purego.RTLD_NOW|purego.RTLD_GLOBAL); err != nil {
			return
		}
		var protocols []*objc.Protocol
		if p := objc.GetProtocol("PHPickerViewControllerDelegate"); p != nil {
			protocols = append(protocols, p)
		}
		classPicker = register("NsgoPickerDelegate", objc.GetClass("NSObject"), protocols, []objc.MethodDef{{
			Cmd: sel("picker:didFinishPicking:"),
			Fn: func(self objc.ID, _ objc.SEL, picker, results objc.ID) {
				picker.Send(sel("dismissViewControllerAnimated:completion:"), true, objc.ID(0))
				req, ok := pickerRequests[self]
				if !ok {
					return
				}
				delete(pickerRequests, self)
				defer self.Send(selRelease)
				s.picked(req, results)
			},
		}})
	})
	if classPicker == 0 {
		s.engine.Dispatch(driver.Event{Kind: driver.EventPick, Count: id})
		return
	}
	config := objc.ID(objc.GetClass("PHPickerConfiguration")).Send(selAlloc).Send(sel("init"))
	defer config.Send(selRelease)
	config.Send(sel("setSelectionLimit:"), max(r.Limit, 1))
	config.Send(sel("setFilter:"), objc.ID(objc.GetClass("PHPickerFilter")).Send(sel("imagesFilter")))
	if objc.Send[bool](config, sel("respondsToSelector:"), sel("setSelection:")) {
		const ordered = 1
		config.Send(sel("setSelection:"), ordered)
	}
	picker := objc.ID(objc.GetClass("PHPickerViewController")).Send(selAlloc).Send(sel("initWithConfiguration:"), config)
	defer picker.Send(selRelease)
	delegate := objc.ID(classPicker).Send(selNew)
	pickerRequests[delegate] = pickRequest{r, id}
	picker.Send(sel("setDelegate:"), delegate)
	s.topController().Send(sel("presentViewController:animated:completion:"), picker, true, objc.ID(0))
}

// picked loads the data of the picker's results and answers with them
// prepared as the request asks.
func (s *surface) picked(req pickRequest, results objc.ID) {
	var providers []objc.ID
	apple.ForEach(results, func(res objc.ID) {
		if len(providers) < max(req.r.Limit, 1) {
			providers = append(providers, res.Send(sel("itemProvider")))
		}
	})
	files := make([]driver.PickedFile, len(providers))
	var wg sync.WaitGroup
	for i, p := range providers {
		files[i].Name = apple.GoString(p.Send(sel("suggestedName")))
		wg.Add(1)
		done := objc.NewBlock(func(_ objc.Block, data, err objc.ID) {
			files[i].Data = apple.Bytes(data)
			wg.Done()
		})
		p.Send(sel("loadDataRepresentationForTypeIdentifier:completionHandler:"), apple.NSString("public.image"), done)
		done.Release()
	}
	go func() {
		wg.Wait()
		var out []driver.PickedFile
		for _, f := range files {
			if len(f.Data) > 0 {
				out = append(out, apple.PreparePicked(req.r, f))
			}
		}
		s.engine.Dispatch(driver.Event{Kind: driver.EventPick, Count: req.id, Data: driver.FinishPick(req.r, out, nil)})
	}()
}

var (
	documentsOnce    sync.Once
	classDocuments   objc.Class
	documentRequests = map[objc.ID]documentRequest{}
)

// documentRequest is a pick or a save a document picker answers.
type documentRequest struct {
	id    uint32
	limit int
	// staged is the directory of the file a save exports, removed once
	// the picker closed; "" for a pick.
	staged string
}

// presentDocuments presents a document picker for req, made by its
// initializer, whose delegate answers it.
func (s *surface) presentDocuments(picker objc.ID, req documentRequest) {
	documentsOnce.Do(func() {
		var protocols []*objc.Protocol
		if p := objc.GetProtocol("UIDocumentPickerDelegate"); p != nil {
			protocols = append(protocols, p)
		}
		answer := func(self objc.ID, urls objc.ID) {
			req, ok := documentRequests[self]
			if !ok {
				return
			}
			delete(documentRequests, self)
			defer self.Send(selRelease)
			ev := driver.Event{Kind: driver.EventFiles, Count: req.id}
			if req.staged != "" {
				os.RemoveAll(req.staged)
				ev.Finished = urls != 0 && objc.Send[int](urls, sel("count")) > 0
			} else if urls != 0 {
				var files []driver.File
				apple.ForEach(urls, func(u objc.ID) {
					if len(files) < max(req.limit, 1) {
						files = append(files, driver.File{Path: apple.GoString(u.Send(sel("path"))), Temporary: true})
					}
				})
				ev.Data = files
			}
			s.engine.Dispatch(ev)
		}
		classDocuments = register("NsgoDocumentPickerDelegate", objc.GetClass("NSObject"), protocols, []objc.MethodDef{{
			Cmd: sel("documentPicker:didPickDocumentsAtURLs:"),
			Fn:  func(self objc.ID, _ objc.SEL, _, urls objc.ID) { answer(self, urls) },
		}, {
			Cmd: sel("documentPickerWasCancelled:"),
			Fn:  func(self objc.ID, _ objc.SEL, _ objc.ID) { answer(self, 0) },
		}})
	})
	defer picker.Send(selRelease)
	delegate := objc.ID(classDocuments).Send(selNew)
	documentRequests[delegate] = req
	picker.Send(sel("setDelegate:"), delegate)
	s.topController().Send(sel("presentViewController:animated:completion:"), picker, true, objc.ID(0))
}

// pickFiles presents UIDocumentPickerViewController for the files r
// offers, which copies the ones picked into the app's temporary
// directory; picking needs no permission.
func (s *surface) pickFiles(r driver.FilesRequest, id uint32) {
	picker := objc.ID(objc.GetClass("UIDocumentPickerViewController")).Send(selAlloc).
		Send(sel("initForOpeningContentTypes:asCopy:"), apple.ContentTypes(r), true)
	picker.Send(sel("setAllowsMultipleSelection:"), r.Limit > 1)
	s.presentDocuments(picker, documentRequest{id: id, limit: r.Limit})
}

// saveFile presents UIDocumentPickerViewController exporting a copy of
// the file r saves, under its name.
func (s *surface) saveFile(r driver.SaveRequest, id uint32) {
	path, err := r.Stage(filepath.Join(os.TempDir(), "nsgo-save"))
	if err != nil {
		s.engine.Dispatch(driver.Event{Kind: driver.EventFiles, Count: id, Text: err.Error()})
		return
	}
	urls := objc.ID(objc.GetClass("NSMutableArray")).Send(sel("array"))
	urls.Send(sel("addObject:"), objc.ID(objc.GetClass("NSURL")).Send(sel("fileURLWithPath:"), apple.NSString(path)))
	picker := objc.ID(objc.GetClass("UIDocumentPickerViewController")).Send(selAlloc).
		Send(sel("initForExportingURLs:asCopy:"), urls, true)
	s.presentDocuments(picker, documentRequest{id: id, staged: filepath.Dir(path)})
}

// setAppearance shows the window, and the system views in it, in a, or
// in the system's appearance.
func setAppearance(a driver.Appearance) {
	if window == 0 {
		return
	}
	window.Send(sel("setOverrideUserInterfaceStyle:"), uiStyle(a))
}

// uiStyle is UIKit's interface style of a, unspecified for the system's.
func uiStyle(a driver.Appearance) int {
	switch a {
	case driver.AppearanceLight:
		return 1
	case driver.AppearanceDark:
		return 2
	}
	return 0
}

// osVersion returns iOS's version.
func osVersion() driver.OSVersion {
	type version struct{ Major, Minor, Patch int }
	v := objc.Send[version](objc.ID(objc.GetClass("NSProcessInfo")).Send(sel("processInfo")), sel("operatingSystemVersion"))
	return driver.OSVersion{Major: v.Major, Minor: v.Minor}
}

// onMac reports whether the app is an iPhone or iPad app running on a
// Mac.
func onMac() bool {
	info := objc.ID(objc.GetClass("NSProcessInfo")).Send(sel("processInfo"))
	return objc.Send[bool](info, sel("respondsToSelector:"), sel("isiOSAppOnMac")) && objc.Send[bool](info, sel("isiOSAppOnMac"))
}
