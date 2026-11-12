//go:build !(darwin && !ios) && !windows && !(linux && !android)

package nsgo

import "github.com/nsgo-dev/nsgo/internal/engine"

func newView(App, options, engine.HostOptions) platformView {
	panic("nsgo: NewView embeds apps in desktop hosts; iOS and Android apps embed them with the frameworks nsgo build -embed makes")
}
