package main

import "github.com/nsgo-dev/nsgo/core"

// components are the gallery's native components, registered in main:
// each platform's file adds its own.
var components = map[string]core.NativeFactory{}

// sliderComponent is the name of the platform's own slider, which each
// platform's file registers where it has one.
const sliderComponent = "gallery.slider"
