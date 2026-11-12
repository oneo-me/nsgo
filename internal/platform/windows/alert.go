//go:build windows

package windows

import (
	"syscall"
	"unsafe"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// Alerts are drawn in the surface, as NSH's Windows backend draws them,
// in the style of Windows 11's content dialog: a rounded card over a
// scrim, its title and message, and its buttons in a band at its foot,
// the default one in the system's accent color; an action sheet's stack.
// They are items the window draws over the views rather than a task
// dialog, so they stay with an embedded surface and never run a message
// loop of their own. While one shows, the pointer and the keys are its:
// Tab and the arrows move between the buttons, Return and Space press
// the focused one, and Escape cancels when the alert can be cancelled.

var (
	mbGetString    = user32.NewProc("MB_GetString")
	getProcAddress = kernel32.NewProc("GetProcAddress")
)

const (
	alertPad     = 24
	alertGap     = 12
	alertButtonH = 32
	alertSpacing = 8
	alertRadius  = 8
	alertMaxW    = 448
	alertMinW    = 320

	vkSpace = 0x20
)

// shownAlert is the alert showing: its config, its texts' layouts and
// where each part went in the last frame, in points.
type shownAlert struct {
	config                driver.AlertConfig
	labels                []string // the buttons'
	title, message        *comObject
	titleH, messageH      float32
	buttonLayouts         []*comObject
	card, band            rectF
	buttons               []rectF
	hover, pressed, focus int // a button's index, -1 for none
	cancel                int // the cancel action's index, -1 for none
	cancelable            bool
}

// showAlert shows the first queued alert unless one is showing.
func (w *window) showAlert() {
	if w.alerting || len(w.alerts) == 0 {
		return
	}
	a := w.alerts[0]
	w.alerts = w.alerts[1:]
	w.alerting, w.alertShown = true, a
	s := &shownAlert{config: a, hover: -1, pressed: -1, focus: -1, cancel: -1}
	for i, act := range a.Actions {
		s.labels = append(s.labels, act.Label)
		if act.Role == driver.AlertCancel && s.cancel < 0 {
			s.cancel = i
		}
		if act.Role == driver.AlertDefault && s.focus < 0 {
			s.focus = i
		}
	}
	if len(s.labels) == 0 {
		// The system's own OK, in the user's language.
		ok := "OK"
		if r, _, _ := mbGetString.Call(0); r != 0 {
			p := *(**uint16)(unsafe.Pointer(&r))
			ok = syscall.UTF16ToString(unsafe.Slice(p, wcslen(p)))
		}
		s.labels = []string{ok}
	}
	if s.focus < 0 {
		s.focus = 0
	}
	s.cancelable = s.cancel >= 0 || a.Cancelable || len(a.Actions) == 0
	w.alert = s
	// The keys are the alert's; the field gets the focus back after.
	if in := w.focusedField(); in != nil {
		w.alertRestore = in
		w.blurAll()
	}
	w.redrawAll()
}

// answerAlert closes the alert with the action at index, -1 for none.
func (w *window) answerAlert(index int) {
	s := w.alert
	if s == nil {
		return
	}
	if len(s.config.Actions) == 0 {
		index = -1
	}
	releaseAll(&s.title, &s.message)
	for i := range s.buttonLayouts {
		releaseAll(&s.buttonLayouts[i])
	}
	w.alert, w.alerting = nil, false
	w.engine.Dispatch(driver.Event{Kind: driver.EventAlert, Count: s.config.ID, Index: index})
	if in := w.alertRestore; in != nil {
		w.alertRestore = nil
		if !in.v.deleted {
			w.setFocus(in)
		}
	}
	w.redrawAll()
	if len(w.alerts) > 0 {
		postMessageW.Call(w.hwnd, wmAlert, 0, 0)
	}
}

// alertText makes a layout of text in font, wrapped at width.
func alertText(text string, font driver.Font, width float32) (*comObject, float32) {
	format := textFormat(font)
	if format == nil || text == "" {
		releaseAll(&format)
		return nil, 0
	}
	defer releaseAll(&format)
	units := syscall.StringToUTF16(text)
	units = units[:len(units)-1]
	var layout *comObject
	if failed(dwriteFactory().callF(dwCreateTextLayout, 0b11000, uintptr(unsafe.Pointer(&units[0])), uintptr(len(units)),
		format.ptr(), f32(width), f32(1e6), uintptr(unsafe.Pointer(&layout)))) {
		return nil, 0
	}
	layout.call(formatSetWordWrapping, dwWrap)
	var m textMetrics
	layout.call(layoutGetMetrics, uintptr(unsafe.Pointer(&m)))
	return layout, m.height
}

// layoutAlert places the alert's parts in a surface of size, making its
// texts' layouts at the card's width.
func (s *shownAlert) layout(size driver.Size) {
	width := float32(min(max(float64(size.Width)-32, alertMinW), alertMaxW))
	inner := width - 2*alertPad
	if s.title == nil && s.message == nil && s.buttonLayouts == nil {
		s.title, s.titleH = alertText(s.config.Title, driver.Font{Size: 20, Weight: driver.FontWeightSemibold}, inner)
		s.message, s.messageH = alertText(s.config.Message, driver.Font{Size: 14}, inner)
		for _, l := range s.labels {
			b, _ := alertText(l, driver.Font{Size: 14}, 1e4)
			if b != nil {
				b.call(formatSetTextAlignment, dwAlignCenter)
				b.call(formatSetParagraphAlignment, dwParagraphCenter)
			}
			s.buttonLayouts = append(s.buttonLayouts, b)
		}
	}
	body := float32(alertPad) + s.titleH
	if s.message != nil {
		if s.title != nil {
			body += alertGap
		}
		body += s.messageH
	}
	body += alertPad
	n := len(s.labels)
	stacked := s.config.Sheet && n > 1
	band := float32(2*alertPad + alertButtonH)
	if stacked {
		band = float32(2*alertPad + n*alertButtonH + (n-1)*alertSpacing)
	}
	left := (float32(size.Width) - width) / 2
	top := max((float32(size.Height)-body-band)/2, 0)
	s.card = rectF{left, top, left + width, top + body + band}
	s.band = rectF{left, top + body, left + width, top + body + band}
	s.buttons = s.buttons[:0]
	for i := range n {
		if stacked {
			y := s.band.top + alertPad + float32(i*(alertButtonH+alertSpacing))
			s.buttons = append(s.buttons, rectF{left + alertPad, y, left + width - alertPad, y + alertButtonH})
			continue
		}
		bw := (inner - float32((n-1)*alertSpacing)) / float32(n)
		x := left + alertPad + float32(i)*(bw+alertSpacing)
		s.buttons = append(s.buttons, rectF{x, s.band.top + alertPad, x + bw, s.band.top + alertPad + alertButtonH})
	}
	for i, l := range s.buttonLayouts {
		if l != nil {
			b := s.buttons[i]
			l.callF(layoutSetMaxWidth, 1, f32(b.right-b.left))
			l.callF(layoutSetMaxHeight, 1, f32(alertButtonH))
		}
	}
}

// accentColor is the system's accent, the default button's fill.
func accentColor() driver.Color {
	abgr := regDWORD(`Software\Microsoft\Windows\CurrentVersion\Explorer\Accent`, "AccentColorMenu", 0xffb85f00)
	return driver.RGB(abgr&0xff<<16 | abgr>>8&0xff<<8 | abgr>>16&0xff)
}

// drawAlert draws the alert showing over the views.
func (p *painter) drawAlert(size driver.Size) {
	s := p.w.alert
	if s == nil {
		return
	}
	s.layout(size)
	dark := p.w.env.Dark
	text, card, band, button, border := driver.RGB(0x1a1a1a), driver.RGB(0xffffff), driver.RGB(0xf3f3f3),
		driver.RGB(0xfbfbfb), driver.RGBA(0x000000, 0.08)
	if dark {
		text, card, band, button, border = driver.RGB(0xffffff), driver.RGB(0x2b2b2b), driver.RGB(0x202020),
			driver.RGB(0x373737), driver.RGBA(0xffffff, 0.08)
	}
	p.setTransform(identity)
	p.fillRect(rectF{0, 0, float32(size.Width), float32(size.Height)}, driver.RGBA(0x000000, 0.3))
	// A hairline border, then the card and its band.
	p.fillRounded(rectF{s.card.left - 1, s.card.top - 1, s.card.right + 1, s.card.bottom + 1}, alertRadius+1, border)
	p.fillRounded(s.card, alertRadius, band)
	// The body's rounded bottom goes under the band.
	p.fillRounded(rectF{s.card.left, s.card.top, s.card.right, s.band.top + alertRadius}, alertRadius, card)
	p.fillRect(rectF{s.card.left, s.band.top, s.card.right, s.band.top + alertRadius}, band)
	p.fillRect(rectF{s.card.left, s.band.top, s.card.right, s.band.top + 1}, border)
	y := s.card.top + alertPad
	if s.title != nil {
		p.dc.callPoint(rtDrawTextLayout, s.card.left+alertPad, y, s.title.ptr(), p.solid(text).ptr(), d2dDrawTextEnableColorFont)
		y += s.titleH + alertGap
	}
	if s.message != nil {
		p.dc.callPoint(rtDrawTextLayout, s.card.left+alertPad, y, s.message.ptr(), p.solid(text).ptr(), d2dDrawTextEnableColorFont)
	}
	accent := accentColor()
	for i, r := range s.buttons {
		fill, label := button, text
		primary := i < len(s.config.Actions) && s.config.Actions[i].Role == driver.AlertDefault || len(s.config.Actions) == 0
		if primary {
			fill, label = accent, driver.RGB(0xffffff)
		}
		if i < len(s.config.Actions) && s.config.Actions[i].Role == driver.AlertDestructive {
			label = driver.RGB(0xc42b1c)
			if dark {
				label = driver.RGB(0xff99a4)
			}
		}
		switch {
		case i == s.pressed && i == s.hover:
			fill = fill&^0xff | fill&0xff*8/10
		case i == s.hover:
			fill = fill&^0xff | fill&0xff*9/10
		}
		if !primary {
			p.fillRounded(rectF{r.left - 1, r.top - 1, r.right + 1, r.bottom + 1}, 5, border)
		}
		p.fillRounded(r, 4, fill)
		if i == s.focus {
			focus := driver.RGB(0x000000)
			if dark {
				focus = driver.RGB(0xffffff)
			}
			p.strokeRounded(rectF{r.left - 3, r.top - 3, r.right + 3, r.bottom + 3}, 7, 2, focus)
		}
		if l := s.buttonLayouts[i]; l != nil {
			p.dc.callPoint(rtDrawTextLayout, r.left, r.top, l.ptr(), p.solid(label).ptr(), d2dDrawTextEnableColorFont)
		}
	}
}

// strokeRounded strokes r's rounded outline width wide.
func (p *painter) strokeRounded(r rectF, radius, width float32, c driver.Color) {
	rr := roundedRect{rect: r, radiusX: radius, radiusY: radius}
	p.dc.callF(rtDrawRoundedRectangle, 0b100, uintptr(unsafe.Pointer(&rr)), p.solid(c).ptr(), f32(width), 0)
}

// alertButtonAt returns the button under p, -1 for none.
func (s *shownAlert) buttonAt(p driver.Point) int {
	for i, r := range s.buttons {
		if float32(p.X) >= r.left && float32(p.X) < r.right && float32(p.Y) >= r.top && float32(p.Y) < r.bottom {
			return i
		}
	}
	return -1
}

// alertMouse takes the pointer while an alert shows; it reports whether
// one does.
func (w *window) alertMouse(message uint32, at driver.Point) bool {
	s := w.alert
	if s == nil {
		return false
	}
	hover := s.buttonAt(at)
	switch message {
	case wmLButtonDown:
		s.pressed = hover
		setCapture.Call(w.hwnd)
	case wmLButtonUp:
		pressed := s.pressed
		s.pressed = -1
		releaseCapture.Call()
		if pressed >= 0 && pressed == hover {
			w.answerAlert(pressed)
			return true
		}
	}
	if hover != s.hover || message != wmMouseMove {
		s.hover = hover
		w.redrawAll()
	}
	return true
}

// alertKey takes the keys while an alert shows; it reports whether one
// does.
func (w *window) alertKey(vk uintptr) bool {
	s := w.alert
	if s == nil {
		return false
	}
	n := len(s.labels)
	switch vk {
	case vkEscape:
		if s.cancelable {
			w.answerAlert(s.cancel)
			return true
		}
	case vkReturn, vkSpace:
		w.answerAlert(s.focus)
		return true
	case vkTab:
		if keyDown(vkShift) {
			s.focus = (s.focus + n - 1) % n
		} else {
			s.focus = (s.focus + 1) % n
		}
	case vkLeft, vkUp:
		s.focus = max(s.focus-1, 0)
	case vkRight, vkDown:
		s.focus = min(s.focus+1, n-1)
	}
	w.redrawAll()
	return true
}
