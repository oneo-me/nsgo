// Package ui is NSGo's built-in styled components, on base and core:
// colors, finishes and motion in theme.go, type and small parts in
// style.go, and a component to a file. A complex component has a package
// below this one, as chat has. Icons are Lucide's accessors.
//
// The look is NSGo's: a neutral page, flat bordered cards under bold
// titles, flat bordered controls 28 pt tall in 13 pt text by default,
// 8 pt corners and one blue accent, the same sizes on every device. Its
// colors are core.DefaultTheme's, which holds every role it reads.
// A component takes its look from Variant; the few of several sizes take
// Size, in size.go, or points where they are one shape at any scale.
//
// Components are structs with a Render method: the app sets the fields
// and handles changes. Each reads its colors where it is built (ColorsOf,
// TintsOf, FinOf), so a subtree's theme or forced appearance reaches it,
// and reads them in StyleFuncs, Draws and the like, which follow a change
// of theme or appearance in place, as NSH's styles do: nothing is built
// again.
package ui
