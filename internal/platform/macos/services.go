//go:build darwin && !ios

package macos

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

// The services AppKit performs: the clipboard's images and files,
// opening URLs and files, the photo and file pickers, the save panel,
// the share picker, context menus, the app's appearance and the
// window's frame and level.

func generalPasteboard() objc.ID {
	return objc.ID(apple.Class("NSPasteboard")).Send(sel("generalPasteboard"))
}

// The pasteboard types of images, in the order a read prefers them.
var imageTypes = []string{"public.png", "public.tiff", "public.jpeg", "public.heic", "com.compuserve.gif"}

// setClipboardImage puts the image data encodes on the pasteboard as a
// PNG and a TIFF, which every Mac app reads.
func setClipboardImage(data []byte) {
	img := objc.ID(apple.Class("NSImage")).Send(selAlloc).Send(sel("initWithData:"), apple.NSData(data))
	if img == 0 {
		return
	}
	defer img.Send(selRelease)
	pb := generalPasteboard()
	pb.Send(sel("clearContents"))
	if png, _, err := apple.PNG(data); err == nil {
		pb.Send(sel("setData:forType:"), apple.NSData(png), apple.NSString("public.png"))
	}
	if tiff := img.Send(sel("TIFFRepresentation")); tiff != 0 {
		pb.Send(sel("setData:forType:"), tiff, apple.NSString("public.tiff"))
	}
}

// availableType returns the first of types the pasteboard holds, "" for
// none, without reading its data.
func availableType(pb objc.ID, types []string) string {
	arr := objc.ID(apple.Class("NSMutableArray")).Send(sel("array"))
	for _, t := range types {
		arr.Send(sel("addObject:"), apple.NSString(t))
	}
	return apple.GoString(pb.Send(sel("availableTypeFromArray:"), arr))
}

// readClipboard answers an ItemReadClipboard.
func (s *surface) readClipboard(it *driver.MountItem) {
	pb := generalPasteboard()
	ev := driver.Event{Kind: driver.EventClipboard, Count: it.Count}
	switch it.Clipboard {
	case driver.ClipboardReadText:
		ev.Text = apple.GoString(pb.Send(sel("stringForType:"), apple.NSString(pasteboardText)))
	case driver.ClipboardReadHasText:
		ev.Finished = availableType(pb, []string{pasteboardText}) != ""
	case driver.ClipboardReadImage:
		if len(fileURLs(pb)) > 0 {
			break // a file's icon is no image of the user's
		}
		if t := availableType(pb, imageTypes); t != "" {
			data := apple.Bytes(pb.Send(sel("dataForType:"), apple.NSString(t)))
			// Converting is slow for photos: off the main thread.
			id := it.Count
			go func() {
				ev := driver.Event{Kind: driver.EventClipboard, Count: id}
				ev.Data = driver.ReadClipboardImage(apple.PNG(data))
				s.engine.Dispatch(ev)
			}()
			return
		}
	case driver.ClipboardReadHasImage:
		ev.Finished = len(fileURLs(pb)) == 0 && availableType(pb, imageTypes) != ""
	case driver.ClipboardReadFiles:
		ev.URIs = fileURLs(pb)
	}
	s.engine.Dispatch(ev)
}

// notify carries out a notification's item through the notification
// center; the badge is the Dock tile's, which needs no permission.
func (s *surface) notify(it *driver.MountItem) {
	e, id := s.engine, it.Count
	switch it.Kind {
	case driver.ItemNotificationPermission:
		apple.NotificationPermission(it.On, func(p int) {
			e.Dispatch(driver.Event{Kind: driver.EventNotification, Count: id, Index: p})
		})
	case driver.ItemShowNotification:
		apple.ShowNotification(*it.Notification, func(shown bool) {
			e.Dispatch(driver.Event{Kind: driver.EventNotification, Count: id, Finished: shown})
		})
	case driver.ItemCloseNotification:
		apple.CloseNotification(it.Value)
	case driver.ItemBadge:
		label := objc.ID(0)
		if it.Count > 0 {
			label = apple.NSString(strconv.FormatUint(uint64(it.Count), 10))
		}
		objc.ID(apple.Class("NSApplication")).Send(sel("sharedApplication")).Send(sel("dockTile")).Send(sel("setBadgeLabel:"), label)
	}
}

// openURL opens u with the app the system picks and reports whether one
// took it.
func (s *surface) openURL(u string, id uint32) {
	nsURL := objc.ID(apple.Class("NSURL")).Send(sel("URLWithString:"), apple.NSString(u))
	opened := false
	if nsURL != 0 {
		ws := objc.ID(apple.Class("NSWorkspace")).Send(sel("sharedWorkspace"))
		opened = objc.Send[bool](ws, sel("openURL:"), nsURL)
	}
	s.engine.Dispatch(driver.Event{Kind: driver.EventOpenURL, Count: id, Finished: opened})
}

// openFile opens f in the app Launch Services associates with it, and
// reports whether one did: none when no app opens its type.
func (s *surface) openFile(f driver.SharedFile, id uint32) {
	u := objc.ID(apple.Class("NSURL")).Send(sel("fileURLWithPath:"), apple.NSString(f.Path))
	ws := objc.ID(apple.Class("NSWorkspace")).Send(sel("sharedWorkspace"))
	ev := driver.Event{Kind: driver.EventFiles, Count: id}
	if ws.Send(sel("URLForApplicationToOpenURL:"), u) != 0 {
		ev.Finished = objc.Send[bool](ws, sel("openURL:"), u)
		if !ev.Finished {
			ev.Text = "macOS could not open " + f.Path
		}
	}
	s.engine.Dispatch(ev)
}

// share shows NSSharingServicePicker for r's files, under their names,
// and text, pointing at r.From.
func (s *surface) share(r driver.ShareRequest, id uint32) {
	items := objc.ID(apple.Class("NSMutableArray")).Send(sel("array"))
	for _, f := range r.Files {
		path, err := f.Stage(filepath.Join(os.TempDir(), "nsgo-share"))
		if err != nil {
			s.engine.Dispatch(driver.Event{Kind: driver.EventFiles, Count: id, Text: err.Error()})
			return
		}
		items.Send(sel("addObject:"), objc.ID(apple.Class("NSURL")).Send(sel("fileURLWithPath:"), apple.NSString(path)))
	}
	if r.Text != "" {
		items.Send(sel("addObject:"), apple.NSString(r.Text))
	}
	if s.sharing != 0 {
		s.sharing.Send(selRelease)
	}
	s.sharing = objc.ID(apple.Class("NSSharingServicePicker")).Send(selAlloc).Send(sel("initWithItems:"), items)
	from := r.From
	if from == (driver.Bounds{}) {
		b := objc.Send[apple.CGRect](s.native, sel("bounds"))
		from.Origin = driver.Point{X: driver.Points(b.Size.Width / 2), Y: driver.Points(b.Size.Height / 2)}
	}
	const edgeMaxY = 3 // NSRectEdgeMaxY: below, the view being flipped
	s.sharing.Send(sel("showRelativeToRect:ofView:preferredEdge:"),
		apple.Rect(float64(from.Origin.X), float64(from.Origin.Y), float64(from.Size.Width), float64(from.Size.Height)),
		s.native, edgeMaxY)
	s.engine.Dispatch(driver.Event{Kind: driver.EventFiles, Count: id, Finished: true})
}

// pickImages shows an open panel as a sheet on the window for the
// images r asks for, then prepares the ones chosen off the main thread.
func (s *surface) pickImages(r driver.PickRequest, id uint32) {
	panel := objc.ID(apple.Class("NSOpenPanel")).Send(sel("openPanel"))
	panel.Send(sel("setCanChooseFiles:"), true)
	panel.Send(sel("setCanChooseDirectories:"), false)
	panel.Send(sel("setAllowsMultipleSelection:"), r.Limit > 1)
	types := objc.ID(apple.Class("NSMutableArray")).Send(sel("array"))
	if r.Format == driver.PickCompatible {
		// HEIC and the others become JPEG, so any image will do.
		types.Send(sel("addObject:"), apple.NSString("public.image"))
	} else {
		for _, ext := range r.DialogExtensions() {
			types.Send(sel("addObject:"), apple.NSString(ext))
		}
	}
	panel.Send(sel("setAllowedFileTypes:"), types)
	panel.Send(sel("retain"))
	done := objc.NewBlock(func(_ objc.Block, response int) {
		defer panel.Send(selRelease)
		var paths []string
		if response == 1 { // NSModalResponseOK
			apple.ForEach(panel.Send(sel("URLs")), func(u objc.ID) {
				paths = append(paths, apple.GoString(u.Send(sel("path"))))
			})
		}
		go func() {
			var files []driver.PickedFile
			for _, p := range paths {
				if len(files) >= max(r.Limit, 1) {
					break
				}
				f, ok := readPicked(p)
				if ok {
					files = append(files, apple.PreparePicked(r, f))
				}
			}
			s.engine.Dispatch(driver.Event{Kind: driver.EventPick, Count: id, Data: driver.FinishPick(r, files, nil)})
		}()
	})
	panel.Send(sel("beginSheetModalForWindow:completionHandler:"), s.window, done)
	done.Release()
}

// pickFiles shows an open panel as a sheet on the window for the files
// r offers and answers with their paths.
func (s *surface) pickFiles(r driver.FilesRequest, id uint32) {
	panel := objc.ID(apple.Class("NSOpenPanel")).Send(sel("openPanel"))
	panel.Send(sel("setCanChooseFiles:"), true)
	panel.Send(sel("setCanChooseDirectories:"), false)
	panel.Send(sel("setAllowsMultipleSelection:"), r.Limit > 1)
	panel.Send(sel("setAllowedContentTypes:"), apple.ContentTypes(r))
	panel.Send(sel("retain"))
	done := objc.NewBlock(func(_ objc.Block, response int) {
		defer panel.Send(selRelease)
		var files []driver.File
		if response == 1 { // NSModalResponseOK
			apple.ForEach(panel.Send(sel("URLs")), func(u objc.ID) {
				files = append(files, driver.File{Path: apple.GoString(u.Send(sel("path")))})
			})
		}
		s.engine.Dispatch(driver.Event{Kind: driver.EventFiles, Count: id, Data: files})
	})
	panel.Send(sel("beginSheetModalForWindow:completionHandler:"), s.window, done)
	done.Release()
}

// pickDirectories shows an open panel as a sheet on the window for
// folders and answers with their paths.
func (s *surface) pickDirectories(r driver.DirectoriesRequest, id uint32) {
	panel := objc.ID(apple.Class("NSOpenPanel")).Send(sel("openPanel"))
	panel.Send(sel("setCanChooseFiles:"), false)
	panel.Send(sel("setCanChooseDirectories:"), true)
	panel.Send(sel("setCanCreateDirectories:"), true)
	panel.Send(sel("setAllowsMultipleSelection:"), r.Limit > 1)
	panel.Send(sel("retain"))
	done := objc.NewBlock(func(_ objc.Block, response int) {
		defer panel.Send(selRelease)
		var dirs []driver.Directory
		if response == 1 { // NSModalResponseOK
			apple.ForEach(panel.Send(sel("URLs")), func(u objc.ID) {
				dirs = append(dirs, driver.Directory{Path: apple.GoString(u.Send(sel("path")))})
			})
		}
		s.engine.Dispatch(driver.Event{Kind: driver.EventFiles, Count: id, Data: dirs})
	})
	panel.Send(sel("beginSheetModalForWindow:completionHandler:"), s.window, done)
	done.Release()
}

// saveFile shows a save panel as a sheet on the window for the file r
// suggests, then saves it where the user chose off the main thread.
func (s *surface) saveFile(r driver.SaveRequest, id uint32) {
	panel := objc.ID(apple.Class("NSSavePanel")).Send(sel("savePanel"))
	panel.Send(sel("setNameFieldStringValue:"), apple.NSString(r.Name))
	panel.Send(sel("setCanCreateDirectories:"), true)
	if t := apple.SaveType(r.Name); t != 0 {
		types := objc.ID(apple.Class("NSMutableArray")).Send(sel("array"))
		types.Send(sel("addObject:"), t)
		panel.Send(sel("setAllowedContentTypes:"), types)
		panel.Send(sel("setAllowsOtherFileTypes:"), true)
	}
	panel.Send(sel("retain"))
	done := objc.NewBlock(func(_ objc.Block, response int) {
		defer panel.Send(selRelease)
		if response != 1 {
			s.engine.Dispatch(driver.Event{Kind: driver.EventFiles, Count: id})
			return
		}
		path := apple.GoString(panel.Send(sel("URL")).Send(sel("path")))
		go func() {
			ev := driver.Event{Kind: driver.EventFiles, Count: id, Finished: true, Data: path}
			if err := r.SaveTo(path); err != nil {
				ev.Finished, ev.Text, ev.Data = false, err.Error(), nil
			}
			s.engine.Dispatch(ev)
		}()
	})
	panel.Send(sel("beginSheetModalForWindow:completionHandler:"), s.window, done)
	done.Release()
}

// popupMenu shows entries as a context menu at p in the root's points.
// AppKit tracks it in a loop of its own, so it opens from the main queue
// after the batch, which keeps applying batches meanwhile.
func (s *surface) popupMenu(entries []driver.MenuEntry, p driver.Point, id uint32) {
	s.ensureMenuTarget()
	menu := s.submenu("", entries)
	s.directMenu(menu)
	// One menu at a time: the one tracking closes first.
	s.closePopup()
	s.popup = menu
	apple.OnMain(func() { s.showPopup(menu, p, id) })
}

// showPopup tracks menu unless another replaced it, once the menu
// tracking before it has returned, then reports it closed.
func (s *surface) showPopup(menu objc.ID, p driver.Point, id uint32) {
	if s.tracking {
		s.nextPopups = append(s.nextPopups, func() { s.showPopup(menu, p, id) })
		return
	}
	if s.popup == menu && !finished {
		s.tracking = true
		menu.Send(sel("popUpMenuPositioningItem:atLocation:inView:"), objc.ID(0),
			apple.CGPoint{X: float64(p.X), Y: float64(p.Y)}, s.native)
		s.tracking = false
	}
	if s.popup == menu {
		s.popup = 0
	}
	menu.Send(selRelease)
	// The chosen item's action is sent before the menu returns.
	s.engine.Dispatch(driver.Event{Kind: driver.EventMenuClosed, Count: id})
	next := s.nextPopups
	s.nextPopups = nil
	for _, fn := range next {
		apple.OnMain(fn)
	}
}

// closePopup closes the context menu that tracks, if any.
func (s *surface) closePopup() {
	if s.popup != 0 {
		s.popup.Send(sel("cancelTrackingWithoutAnimation"))
		s.popup = 0
	}
}

// setAppearance shows the app, AppKit's own chrome and the views it
// hosts in a, or in the system's appearance.
func setAppearance(a driver.Appearance) { nsApp().Send(sel("setAppearance:"), nsAppearance(a)) }

// applyAppearance shows the surface in the appearance the app forces:
// the whole app when it runs alone, else its view, in the one the host
// sets where the app forces none.
func (s *surface) applyAppearance() {
	if !s.embedded {
		setAppearance(s.appearance)
		return
	}
	a := s.appearance
	if a == driver.AppearanceSystem {
		a = s.host
	}
	s.native.Send(sel("setAppearance:"), nsAppearance(a))
}

// nsAppearance is AppKit's appearance of a, nil for the system's.
func nsAppearance(a driver.Appearance) objc.ID {
	switch a {
	case driver.AppearanceLight:
		return objc.ID(apple.Class("NSAppearance")).Send(sel("appearanceNamed:"), apple.NSString("NSAppearanceNameAqua"))
	case driver.AppearanceDark:
		return objc.ID(apple.Class("NSAppearance")).Send(sel("appearanceNamed:"), apple.NSString("NSAppearanceNameDarkAqua"))
	}
	return 0
}

func nsApp() objc.ID { return objc.ID(apple.Class("NSApplication")).Send(sel("sharedApplication")) }

// setWindowFrame moves and sizes the window. f's corner is from the main
// screen's top left, y down, and AppKit's frames from its bottom left, y
// up, so the top edge stays where it is unless f moves it.
func (s *surface) setWindowFrame(f driver.WindowFrame) {
	w := s.window
	frame := objc.Send[apple.CGRect](w, sel("frame"))
	top := frame.Origin.Y + frame.Size.Height
	if f.Width > 0 {
		frame.Size.Width = float64(f.Width)
	}
	if f.Height > 0 {
		frame.Size.Height = float64(f.Height)
	}
	// AppKit limits only the user's resizes.
	content := objc.Send[apple.CGRect](w, sel("contentRectForFrameRect:"), frame).Size
	size := driver.Size{Width: driver.Points(content.Width), Height: driver.Points(content.Height)}
	fit := s.constraints.Clamp(size)
	frame.Size.Width += float64(fit.Width - size.Width)
	frame.Size.Height += float64(fit.Height - size.Height)
	if f.Move {
		screens := objc.ID(apple.Class("NSScreen")).Send(sel("screens"))
		if objc.Send[uint](screens, sel("count")) > 0 {
			main := objc.Send[apple.CGRect](screens.Send(sel("objectAtIndex:"), uint(0)), sel("frame"))
			top = main.Origin.Y + main.Size.Height - float64(f.Y)
			frame.Origin.X = main.Origin.X + float64(f.X)
		}
	}
	frame.Origin.Y = top - frame.Size.Height
	w.Send(sel("setFrame:display:animate:"), frame, true, f.Animated)
}

// windowFrame returns the window's frame from the main screen's top left,
// as setWindowFrame takes it.
func (s *surface) windowFrame() driver.WindowFrame {
	if s.embedded || s.window == 0 {
		return driver.WindowFrame{}
	}
	frame := objc.Send[apple.CGRect](s.window, sel("frame"))
	screens := screenFrames("frame")
	if len(screens) == 0 {
		return driver.WindowFrame{}
	}
	main := screens[0]
	return driver.WindowFrame{X: driver.Points(frame.Origin.X - main.Origin.X),
		Y: driver.Points(main.Origin.Y + main.Size.Height - frame.Origin.Y - frame.Size.Height), Move: true,
		Width: driver.Points(frame.Size.Width), Height: driver.Points(frame.Size.Height)}
}

// screenFrames returns each screen's rectangle named by selector, the
// main screen first, in AppKit's coordinates.
func screenFrames(selector string) []apple.CGRect {
	screens := objc.ID(apple.Class("NSScreen")).Send(sel("screens"))
	frames := make([]apple.CGRect, objc.Send[uint](screens, sel("count")))
	for i := range frames {
		frames[i] = objc.Send[apple.CGRect](screens.Send(sel("objectAtIndex:"), uint(i)), sel(selector))
	}
	return frames
}

// displays returns AppKit's screens in the coordinate space used by
// WindowFrame: the primary screen's top left, with y growing down.
func displays() []driver.DisplayInfo {
	screens := objc.ID(apple.Class("NSScreen")).Send(sel("screens"))
	count := objc.Send[uint](screens, sel("count"))
	if count == 0 {
		return nil
	}
	primary := objc.ID(apple.Class("NSScreen")).Send(sel("mainScreen"))
	primaryFrame := objc.Send[apple.CGRect](primary, sel("frame"))
	top := primaryFrame.Origin.Y + primaryFrame.Size.Height
	out := make([]driver.DisplayInfo, 0, count)
	for i := uint(0); i < count; i++ {
		screen := screens.Send(sel("objectAtIndex:"), i)
		frame := objc.Send[apple.CGRect](screen, sel("frame"))
		visible := objc.Send[apple.CGRect](screen, sel("visibleFrame"))
		description := screen.Send(sel("deviceDescription"))
		number := description.Send(sel("objectForKey:"), apple.NSString("NSScreenNumber"))
		id := strconv.FormatUint(uint64(objc.Send[uint64](number, sel("unsignedLongLongValue"))), 10)
		if number == 0 {
			id = strconv.FormatUint(uint64(i), 10)
		}
		point := func(r apple.CGRect) driver.Bounds {
			return driver.Bounds{Origin: driver.Point{X: driver.Points(r.Origin.X - primaryFrame.Origin.X), Y: driver.Points(top - r.Origin.Y - r.Size.Height)}, Size: driver.Size{Width: driver.Points(r.Size.Width), Height: driver.Points(r.Size.Height)}}
		}
		out = append(out, driver.DisplayInfo{ID: id, Frame: point(frame), VisibleFrame: point(visible), Scale: float32(objc.Send[float64](screen, sel("backingScaleFactor"))), Main: screen == primary})
	}
	return out
}

// usableScreens returns what the screens leave windows, past the menu bar
// and the Dock, from the main screen's top left, y down.
func usableScreens() []driver.Bounds {
	frames := screenFrames("visibleFrame")
	screens := make([]driver.Bounds, len(frames))
	if len(frames) == 0 {
		return screens
	}
	main := screenFrames("frame")[0]
	top := main.Origin.Y + main.Size.Height
	for i, f := range frames {
		screens[i] = driver.Bounds{
			Origin: driver.Point{X: driver.Points(f.Origin.X - main.Origin.X), Y: driver.Points(top - f.Origin.Y - f.Size.Height)},
			Size:   driver.Size{Width: driver.Points(f.Size.Width), Height: driver.Points(f.Size.Height)},
		}
	}
	return screens
}

// The window levels and collection behaviors of setLevel.
const (
	levelNormal, levelFloating = 0, 3
	desktopIconLevelKey        = 18 // kCGDesktopIconWindowLevelKey
	joinAllSpaces              = 1 << 0
	stationary                 = 1 << 4
	ignoresCycle               = 1 << 6
)

var cgWindowLevelForKey func(key int32) int32

// setLevel puts the window at level. The desktop's level keeps it on
// every space, out of Mission Control and Cmd-`, as desktop widgets are.
func (s *surface) setLevel(level driver.WindowLevel) {
	w := s.window
	behavior := objc.Send[uint](w, sel("collectionBehavior")) &^ (joinAllSpaces | stationary | ignoresCycle)
	switch level {
	case driver.WindowLevelFloating:
		w.Send(sel("setLevel:"), levelFloating)
	case driver.WindowLevelDesktop:
		if cgWindowLevelForKey == nil {
			cg, err := purego.Dlopen("/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics", purego.RTLD_NOW)
			if err != nil {
				return
			}
			purego.RegisterLibFunc(&cgWindowLevelForKey, cg, "CGWindowLevelForKey")
		}
		w.Send(sel("setLevel:"), int(cgWindowLevelForKey(desktopIconLevelKey)))
		behavior |= joinAllSpaces | stationary | ignoresCycle
	default:
		w.Send(sel("setLevel:"), levelNormal)
	}
	w.Send(sel("setCollectionBehavior:"), behavior)
}

// orderFront shows the window, restored when minimized or the app was
// hidden, at the front of its level, and activates the app.
func (s *surface) orderFront() {
	w := s.window
	if objc.Send[bool](w, sel("isMiniaturized")) {
		w.Send(sel("deminiaturize:"), objc.ID(0))
	}
	nsApp().Send(sel("unhide:"), objc.ID(0))
	activateApp()
	w.Send(sel("makeKeyAndOrderFront:"), objc.ID(0))
}

// osVersion returns macOS's version.
func osVersion() driver.OSVersion {
	type version struct{ Major, Minor, Patch int }
	v := objc.Send[version](objc.ID(apple.Class("NSProcessInfo")).Send(sel("processInfo")), sel("operatingSystemVersion"))
	return driver.OSVersion{Major: v.Major, Minor: v.Minor}
}

// readPicked reads the file at path as a picked file.
func readPicked(path string) (driver.PickedFile, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return driver.PickedFile{}, false
	}
	f := driver.PickedFile{Name: filepath.Base(path), Data: data}
	if info, err := os.Stat(path); err == nil {
		f.Modified = info.ModTime()
	}
	return f, true
}

// pickScreenColor shows the system's color sampler, answering id.
func (s *surface) pickScreenColor(id uint32) {
	sampler := objc.ID(apple.Class("NSColorSampler")).Send(sel("alloc")).Send(sel("init"))
	done := objc.NewBlock(func(_ objc.Block, color objc.ID) {
		defer sampler.Send(selRelease)
		if color == 0 {
			s.engine.Dispatch(driver.Event{Kind: driver.EventFiles, Count: id})
			return
		}
		srgb := objc.ID(apple.Class("NSColorSpace")).Send(sel("sRGBColorSpace"))
		c := color.Send(sel("colorUsingColorSpace:"), srgb)
		part := func(name string) uint32 {
			v := objc.Send[float64](c, sel(name))
			return uint32(max(min(v, 1), 0)*255 + 0.5)
		}
		packed := part("redComponent")<<24 | part("greenComponent")<<16 | part("blueComponent")<<8 | part("alphaComponent")
		s.engine.Dispatch(driver.Event{Kind: driver.EventFiles, Count: id, Finished: true, Data: driver.Color(packed)})
	})
	sampler.Send(sel("showSamplerWithSelectionHandler:"), done)
}
