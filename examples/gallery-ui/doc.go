// Package ui is the gallery's look and the components it shares, as NSH's
// nsh-gallery-ui is its gallery's: colors, finishes and motion in
// theme.go, line icons in icons.go, the components in files by kind, and
// the parts only the gallery's demos use in gallery.go. It composes base
// and core and is maintained with the gallery, not published as an API.
//
// Onexa's components are a copy of this package: its tools/syncgallery
// copies every file but this one and gallery.go, which it excludes. Keep
// the components product-neutral, and keep what only the demos need in an
// excluded file the others do not depend on.
//
// The look is Onevra's gallery: a warm page, demos on flat bordered cards
// under bold titles, flat bordered controls 28 pt tall in 13 pt text,
// 8 pt corners and one terracotta accent. On touch screens controls keep
// that look and take presses 44 pt across.
//
// Components are structs with a Render method: the app sets the fields
// and handles changes. Each reads its colors where it is built (ColorsOf,
// TintsOf, FinOf), so a subtree's theme or forced appearance reaches it,
// and reads them in StyleFuncs, Draws and the like, which follow a change
// of theme or appearance in place, as NSH's styles do: nothing is built
// again.
package ui
