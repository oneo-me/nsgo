// The library that nsgo build puts among the frameworks of apps with
// widgets: package widget calls it through purego to reload a kind's
// widgets, as WidgetKit's WidgetCenter has no Objective-C interface.

import WidgetKit

@_cdecl("nsgo_widgets_reload")
public func nsgoWidgetsReload(_ kind: UnsafePointer<CChar>) {
    WidgetCenter.shared.reloadTimelines(ofKind: String(cString: kind))
}
