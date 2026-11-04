// Command embedded builds the embeddable app in app for iOS and Android
// hosts: `nsgo build -embed -target ios` makes the framework
// examples/embed-ios links, and `-target android` the library
// examples/embed-android adds. Run on its own it is a standalone app.
package main

import (
	"github.com/oneo-me/nsgo"
	"github.com/oneo-me/nsgo/examples/embedded/app"
)

func main() { nsgo.Run(app.Root, nsgo.WindowSize(420, 560)) }
