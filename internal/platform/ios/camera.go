//go:build ios

package ios

import (
	"log"
	"sync"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/apple"
)

// Taking a photo: UIImagePickerController's camera, after the user
// allowed it through AVCaptureDevice, as the picker itself shows only a
// notice when they have not.

const (
	avFoundationPath = "/System/Library/Frameworks/AVFoundation.framework/AVFoundation"
	// UIImagePickerControllerSourceTypeCamera.
	sourceCamera = 1
	// AVAuthorizationStatus.
	cameraNotAsked, cameraAllowed = 0, 3
)

var (
	cameraOnce     sync.Once
	classCamera    objc.Class
	uiImageJPEG    func(image objc.ID, quality float64) objc.ID
	cameraRequests = map[objc.ID]pickRequest{}
)

// cameraAvailable is Environment.Camera: the device has a camera the
// picker shows, which the simulator has not.
var cameraAvailable = sync.OnceValue(func() bool {
	return objc.Send[bool](objc.ID(objc.GetClass("UIImagePickerController")), sel("isSourceTypeAvailable:"), sourceCamera)
})

// takePhoto has the user allow the camera, asking once, then presents it
// for one photo; a refusal or an Info.plist without
// NSCameraUsageDescription, for which iOS would end the app as it asks,
// answers with why.
func (s *surface) takePhoto(r driver.PickRequest, id uint32) {
	fail := func(why string) { s.engine.Dispatch(driver.Event{Kind: driver.EventPick, Count: id, Text: why}) }
	if !cameraAvailable() {
		fail("this device has no camera")
		return
	}
	if !apple.DeclaresUsage("NSCameraUsageDescription") {
		log.Print("nsgo: taking a photo needs NSCameraUsageDescription in the app's Info.plist (nsgo.yaml's apple.info)")
		fail("the app declares no NSCameraUsageDescription")
		return
	}
	if _, err := purego.Dlopen(avFoundationPath, purego.RTLD_NOW|purego.RTLD_GLOBAL); err != nil {
		fail("AVFoundation is missing")
		return
	}
	device, video := objc.ID(objc.GetClass("AVCaptureDevice")), apple.Global(avFoundationPath, "AVMediaTypeVideo")
	switch objc.Send[int](device, sel("authorizationStatusForMediaType:"), video) {
	case cameraAllowed:
		s.presentCamera(r, id)
	case cameraNotAsked:
		block := objc.NewBlock(func(_ objc.Block, granted bool) {
			apple.OnMain(func() {
				if granted {
					s.presentCamera(r, id)
				} else {
					fail("camera access denied")
				}
			})
		})
		device.Send(sel("requestAccessForMediaType:completionHandler:"), video, block)
		block.Release()
	default:
		fail("camera access denied")
	}
}

// presentCamera presents the camera, whose delegate answers id with the
// photo taken, prepared off the main thread, or with none for a cancel.
func (s *surface) presentCamera(r driver.PickRequest, id uint32) {
	cameraOnce.Do(func() {
		if lib, err := purego.Dlopen(uikitPath, purego.RTLD_NOW); err == nil {
			purego.RegisterLibFunc(&uiImageJPEG, lib, "UIImageJPEGRepresentation")
		}
		var protocols []*objc.Protocol
		for _, name := range []string{"UIImagePickerControllerDelegate", "UINavigationControllerDelegate"} {
			if p := objc.GetProtocol(name); p != nil {
				protocols = append(protocols, p)
			}
		}
		answer := func(self, picker, info objc.ID) {
			picker.Send(sel("dismissViewControllerAnimated:completion:"), true, objc.ID(0))
			req, ok := cameraRequests[self]
			if !ok {
				return
			}
			delete(cameraRequests, self)
			defer self.Send(selRelease)
			var data []byte
			if info != 0 && uiImageJPEG != nil {
				// The image's orientation goes into the JPEG, which
				// PreparePicked turns upright as it encodes it again.
				if img := info.Send(sel("objectForKey:"), apple.NSString("UIImagePickerControllerOriginalImage")); img != 0 {
					data = apple.Bytes(uiImageJPEG(img, 1))
				}
			}
			if len(data) == 0 {
				s.engine.Dispatch(driver.Event{Kind: driver.EventPick, Count: req.id})
				return
			}
			go func() {
				// Always encode again, upright and at the request's quality.
				r := req.r
				r.Quality = r.JPEGQuality()
				f := apple.PreparePicked(r, driver.PickedFile{Name: "photo.jpg", Data: data})
				s.engine.Dispatch(driver.Event{Kind: driver.EventPick, Count: req.id, Data: []driver.PickedFile{f}})
			}()
		}
		classCamera = register("NsgoCameraDelegate", objc.GetClass("NSObject"), protocols, []objc.MethodDef{{
			Cmd: sel("imagePickerController:didFinishPickingMediaWithInfo:"),
			Fn:  func(self objc.ID, _ objc.SEL, picker, info objc.ID) { answer(self, picker, info) },
		}, {
			Cmd: sel("imagePickerControllerDidCancel:"),
			Fn:  func(self objc.ID, _ objc.SEL, picker objc.ID) { answer(self, picker, 0) },
		}})
	})
	picker := objc.ID(objc.GetClass("UIImagePickerController")).Send(selAlloc).Send(sel("init"))
	defer picker.Send(selRelease)
	picker.Send(sel("setSourceType:"), sourceCamera)
	delegate := objc.ID(classCamera).Send(selNew)
	cameraRequests[delegate] = pickRequest{r, id}
	picker.Send(sel("setDelegate:"), delegate)
	s.topController().Send(sel("presentViewController:animated:completion:"), picker, true, objc.ID(0))
}
