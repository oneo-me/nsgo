package core

import (
	"log"
	"strings"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// Access says what code under a scope may reach (NSH's nsh_core::access):
// a host gives it to code it did not write, such as a plugin, with
// Scope.ProvideAccess. Without one, as for an app's own code, everything
// is allowed. Core checks it where each request is made, whoever makes
// it, so a Go component showing the code's data is held to it too:
// images, videos and web views refused show nothing and report OnError,
// and the services fail as AccessDenied or answer as the system would
// with nothing there.
type Access interface {
	// Allows reports whether req is granted. It is called on the engine's
	// goroutine, so it may read only what does not change.
	Allows(req AccessRequest) bool
	// WebHosts are the hosts a web view's pages may navigate to at the
	// top level: "*", "*.example.com" or a host. The platform refuses
	// other navigations as they start.
	WebHosts() []string
}

// AccessRequest is what Access is asked for.
type AccessRequest struct {
	// Permission is the capability asked for: AccessNetwork, ...
	Permission string
	// Resource is what is asked of it: a URL, or a host handler's name;
	// "" for a capability with none.
	Resource string
	// Method is a network request's HTTP method, or AccessInput for a web
	// view's trusted input.
	Method string
}

// The capabilities Access is asked for, by NSH's keys.
const (
	// AccessNetwork is fetch and the images and videos loaded from the
	// network, by URL.
	AccessNetwork = "network"
	// AccessWebView is the pages a web view shows, by URL, and with
	// AccessInput the trusted input Ref.SendInput gives them.
	AccessWebView = "webview"
	// AccessOpenURL is OpenURL, by URL.
	AccessOpenURL = "openUrl"
	// AccessClipboard is reading and writing the system clipboard.
	AccessClipboard = "clipboard"
	// AccessPicker is the system's photo, camera and file pickers.
	AccessPicker = "picker"
	// AccessSpeech is the microphone and the speech recognizer.
	AccessSpeech = "speech"
	// AccessStatusBar is setting the status bar.
	AccessStatusBar = "statusBar"
	// AccessNotifications is system notifications and the app's badge.
	AccessNotifications = "notifications"
	// AccessBiometrics is Face ID, Touch ID and device biometric unlock.
	AccessBiometrics = "biometrics"
	// AccessHost is the host's handlers, by name (CallHost, PostHost).
	AccessHost = "host"
)

// AccessInput is AccessRequest.Method for a web view's trusted input.
const AccessInput = "input"

// accessKey is where ProvideAccess keeps an owner's Access.
type accessKey struct{}

// ProvideAccess holds the current owner and what it owns to a: what they
// render and the services they call, until the owner is disposed. An
// owner below with an Access of its own is held to both.
func (cx *Scope) ProvideAccess(a Access) {
	if a == nil {
		return
	}
	o := cx.current()
	if outer := accessOf(o); outer != nil {
		a = bothAccess{outer, a}
	}
	if o.values == nil {
		o.values = map[any]any{}
	}
	o.values[accessKey{}] = a
	cx.rt.restricted = true
}

// CheckAccess returns AccessDenied when the current owner's Access
// refuses req, for the requests core does not make itself, such as a
// fetch of tsx's.
func (cx *Scope) CheckAccess(req AccessRequest) error {
	if !cx.rt.restricted {
		return nil
	}
	return refusal(accessOf(cx.current()), req)
}

// accessOf is the Access nearest o, nil for none.
func accessOf(o *Scope) Access {
	if o == nil || !o.rt.restricted {
		return nil
	}
	for s := o; s != nil; s = s.parent {
		if a, ok := s.values[accessKey{}]; ok {
			return a.(Access)
		}
	}
	return nil
}

// refusal is AccessDenied when a refuses req.
func refusal(a Access, req AccessRequest) error {
	if a == nil || a.Allows(req) {
		return nil
	}
	resource := req.Resource
	if req.Method == AccessInput {
		resource = AccessInput
	}
	return AccessDenied(req.Permission, resource)
}

// denied reports whether the current owner's Access refuses req,
// logging the first refusal of each capability.
func (cx *Scope) denied(req AccessRequest) bool {
	err := cx.CheckAccess(req)
	if err == nil {
		return false
	}
	if !cx.rt.refusals[req.Permission] {
		if cx.rt.refusals == nil {
			cx.rt.refusals = map[string]bool{}
		}
		cx.rt.refusals[req.Permission] = true
		log.Printf("nsgo: %v", err)
	}
	return true
}

// bothAccess allows what both allow.
type bothAccess struct{ outer, inner Access }

func (b bothAccess) Allows(req AccessRequest) bool { return b.outer.Allows(req) && b.inner.Allows(req) }

func (b bothAccess) WebHosts() []string {
	return driver.IntersectHosts(b.outer.WebHosts(), b.inner.WebHosts())
}

// sourceRefusal is the refusal of what a node of owner o loads from uri:
// one from the network needs permission's grant, as fetch does, and a
// file on the device is refused. Bytes the app holds (data:, blob:) and
// its assets need nothing.
func sourceRefusal(o *Scope, permission, uri string) error {
	a := accessOf(o)
	if a == nil || uri == "" {
		return nil
	}
	scheme, _, _ := strings.Cut(uri, ":")
	switch strings.ToLower(scheme) {
	case "data", "blob", "asset", "about":
		return nil
	case "http", "https":
		return refusal(a, AccessRequest{Permission: permission, Resource: uri, Method: "GET"})
	}
	if _, ok := o.rt.webSchemes[strings.ToLower(scheme)]; ok {
		return nil
	}
	return AccessDenied(permission, uri)
}

// refuseSource empties *uri when the Access of n's owner refuses it, and
// has report hear each refused URI once, after the commit.
func (t *Tree) refuseSource(n *node, permission string, uri, refused *string, report func(error)) {
	err := sourceRefusal(n.owner, permission, *uri)
	if err == nil {
		return
	}
	if *refused != *uri && report != nil && t.rt.post != nil {
		owner := n.owner
		t.rt.post(func() {
			if !owner.disposed {
				t.within(owner, func() { report(err) })
			}
		})
	}
	*refused, *uri = *uri, ""
}

// refuseImage empties the image sources n's Access refuses; one the app
// holds the bytes of loads nothing.
func (t *Tree) refuseImage(n *node, src *ImageSource, refused *string, report func(error)) {
	if len(src.Data) > 0 {
		return
	}
	t.refuseSource(n, AccessNetwork, &src.URI, refused, report)
	if src.URI == "" {
		src.Variants = nil
		return
	}
	var kept map[float32]string
	for scale, uri := range src.Variants {
		if sourceRefusal(n.owner, AccessNetwork, uri) == nil {
			if kept == nil {
				kept = map[float32]string{}
			}
			kept[scale] = uri
		}
	}
	src.Variants = kept
}
