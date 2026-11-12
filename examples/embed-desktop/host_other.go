//go:build (!darwin && !windows && !linux) || ios || android

package main

import "github.com/nsgo-dev/nsgo/core"

func runHost() { panic("embed-desktop: a desktop host runs on macOS, Windows and Linux") }

func newBadge(*core.NativeContext) core.NativeComponent { return nil }
