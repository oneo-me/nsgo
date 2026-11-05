// Package ui is NSGo's built-in styled components, on base and core:
// colors, finishes and motion in theme.go, type and small parts in
// style.go, and a component to a file. A complex component has a package
// below this one, as chat has. Icons are Lucide's accessors.
//
// The look is Onevra's: a warm page, demos on flat bordered cards under
// bold titles, flat bordered controls 28 pt tall in 13 pt text, 8 pt
// corners and one terracotta accent. On touch screens controls keep that
// look and take presses 44 pt across.
//
// Components are structs with a Render method: the app sets the fields
// and handles changes. Each reads its colors where it is built (ColorsOf,
// TintsOf, FinOf), so a subtree's theme or forced appearance reaches it,
// and reads them in StyleFuncs, Draws and the like, which follow a change
// of theme or appearance in place, as NSH's styles do: nothing is built
// again.
package ui
