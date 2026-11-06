# Device differences

Every place a component in `base` or `ui` changes on its own by device, without the app asking. Read this before branching a component on `Env.Desktop`, `Env.Idiom`, `Env.OS` or a width, and add the branch here.

## Rules

- Keep a control's size and look the same on every device. Never grow a control or its press area for a finger; a resize handle's 3-point band is the [one exception](#touch-input).
- Let the app ask for a different size or form through a field or style.
- Branch only for what a device cannot do, such as hover without a pointer, or for a form listed under [presentation](#presentation).
- Use `ui.Touch` for a branch on input. Use `Env.Idiom` or `Env.OS` only when touch is not the reason.

Sizes are constants for this reason: `ui.ButtonHeight` is 28 points and `ui.RowHeight` 36 on every device. A phone's controls are therefore smaller than the platforms' 44-point guidance unless the app sizes them.

## Presentation

These swap one form for another. Each is an [open question](../../ARCHITECTURE.md#open-questions) until the app can choose.

| Component | Desktop | Touch screen | Decided by |
| --- | --- | --- | --- |
| `ui.Dialog` | Centered dialog, footer in a row | Bottom sheet, footer stacked | `ui.Touch` |
| `base.Menu`, `ui.Select` | Popup at the trigger, submenus cascade | Bottom sheet, submenus replace the items | `Env.Desktop` |
| `base.ContextMenu` | Popup on a secondary click | Sheet on a long press | `Env.Desktop` |
| `ui.NumberInput` | Stacked chevrons, 22 points wide | A minus and a plus, each 28 points wide | `ui.Touch` |
| `base/dock.Area` | Docked panels | One group at a time, docks as drawers | Width under 600 points, unless `Compact` is set |

`ui.MenuItem` rows follow the sheet: 28 points in a popup and 44 across a sheet, which also hides `Shortcut`.

## Hover and pointer

Touch screens have no pointer, so these exist on desktops alone.

| Component | Desktop only |
| --- | --- |
| `base.Tooltip`, `View.Tooltip` | Opening on hover; the app can still open a controlled tooltip on touch screens |
| `ui.Rating` | The hovered star preview |
| `ui.AvatarGroup` | The fan on hover and each member's tooltip |
| `ui.MenuItem` | Row hover in a popup |
| `base/diff` | A drag over the line numbers selects lines |
| `base/datatable` | Its own scroll bars; touch screens show the system's indicator |

`ui.HandleThumb` covers the same gap the other way. Phones have no resize cursor, so a handle shows a pill while pressed.

## Touch input

| Component | Desktop | Touch screen |
| --- | --- | --- |
| `base.ResizeHandle`, `base.ResizablePanelGroup` | 3-point band | 10 more points either side, since 3 points cannot take a finger |
| `base.Popover` | A press outside closes and goes on | A transparent backdrop takes the press |
| `base/dock` tab drag | Starts after moving | Starts after a long press |
| `base/editor` | Press places the caret, drag selects | Tap, long press and selection handles |
| `base/terminal` | No key bar | A bar of keys over the software keyboard, unless `NoKeyBar` is set |

## Keyboard and focus

| Component | Desktop | Touch screen |
| --- | --- | --- |
| `ui.List` | One Tab stop with a focus ring; a double press activates | No focus stop; a tap activates, or toggles in `Multiple` |
| `ui.Slider`, `ui.Rating` | A press focuses and rings the control | No focus |
| `ui/chat` effort picker | Takes focus as it opens | Leaves the focus alone |
| `ui/chat` composer | Return submits | Return adds a line |

## System look

| Component | Differs on | What differs |
| --- | --- | --- |
| `ui.TabShell` | iOS | Floats over the content when it uses the system's tab bar |
| `ui/chat` glass surfaces | macOS, iOS, Windows | No drawn rim, as the system's glass draws its own |
| `ui/filepicker` | macOS | Shortcuts use Command instead of Control |
