// The Live Activities of the widget extension: one ActivityConfiguration
// for every kind, drawing the views of each place the content-state holds
// as timelines' views are drawn, its images in the activity's directory of
// the app group's container. Kinds.swift adds it to the bundle in apps
// with live_activities.

#if os(iOS)
import ActivityKit
import SwiftUI
import WidgetKit

/// WActivity is a content-state as package widget writes it: the fields
/// of the activity and the views the app gave in their place.
struct WActivity: Decodable {
    struct Action: Decodable {
        let id, title: String
        let symbol: String?
    }

    let v: Int
    let title: String
    let body, status, symbol, url: String?
    let timer, progress: Double?
    let actions: [Action]?
    let tint, bg: WColor?
    let lock, leading, trailing, center, bottom, cl, ct, min: WNode?

    static func decode(_ state: JSONValue) -> WActivity? {
        guard let data = try? JSONEncoder().encode(state), let a = try? JSONDecoder().decode(WActivity.self, from: data),
              a.v <= activityVersion
        else { return nil }
        return a
    }
}

/// The version of content-states this extension reads.
let activityVersion = 1

func activityDirectory(_ id: String) -> URL? {
    kindDirectory(".activities")?.appendingPathComponent(id, isDirectory: true)
}

@available(iOS 16.1, *)
struct NSGoActivityWidget: Widget {
    var body: some WidgetConfiguration {
        ActivityConfiguration(for: NSGoActivityAttributes.self) { context in
            ActivityLockScreen(activity: WActivity.decode(context.state.value), id: context.attributes.id)
        } dynamicIsland: { context in
            let a = WActivity.decode(context.state.value)
            let d = Defaults(activity: a, id: context.attributes.id)
            return DynamicIsland {
                DynamicIslandExpandedRegion(.leading) { d.place(a?.leading) { d.symbol(.title2).padding(.leading, 4) } }
                DynamicIslandExpandedRegion(.trailing) {
                    d.place(a?.trailing) { d.status(.title3).minimumScaleFactor(0.6).padding(.trailing, 4) }
                }
                DynamicIslandExpandedRegion(.center) { d.place(a?.center) { d.titles } }
                DynamicIslandExpandedRegion(.bottom) {
                    d.place(a?.bottom) {
                        VStack(spacing: 10) {
                            d.bar
                            d.buttons
                        }
                    }
                }
            } compactLeading: {
                d.place(a?.cl) { d.badge(20) }
            } compactTrailing: {
                d.place(a?.ct) { d.status(.body).minimumScaleFactor(0.6).frame(maxWidth: a?.timer == nil ? nil : 52) }
            } minimal: {
                d.place(a?.min) { d.badge(nil) }
            }
            .keylineTint(a?.tint?.color(.dark))
            .widgetURL(a?.url.flatMap { URL(string: $0) })
        }
    }
}

/// Defaults draws what an activity's fields say in a place it gave no
/// view, and the view it gave elsewhere.
struct Defaults {
    let activity: WActivity?
    let id: String

    var tint: Color? { activity?.tint?.color(.dark) }

    @ViewBuilder func place<V: View>(_ node: WNode?, _ fallback: () -> V) -> some View {
        if let node {
            NodeView(node: node, dir: activityDirectory(id)).environment(\.nsgoActivity, id)
        } else if activity != nil {
            fallback()
        }
    }

    @ViewBuilder func symbol(_ style: Font.TextStyle) -> some View {
        if let name = activity?.symbol {
            Image(systemName: name).font(.system(style).weight(.semibold)).foregroundColor(tint)
        }
    }

    /// badge is the symbol, else a ring of the progress, size points across.
    @ViewBuilder func badge(_ size: CGFloat?) -> some View {
        if activity?.symbol != nil {
            symbol(.body)
        } else if let p = activity?.progress {
            ProgressRing(value: p).foregroundColor(tint).frame(width: size, height: size)
        } else if size == nil {
            status(.caption).minimumScaleFactor(0.5)
        }
    }

    @ViewBuilder func status(_ style: Font.TextStyle) -> some View {
        let font = Font.system(style).weight(.semibold).monospacedDigit()
        if let t = activity?.timer {
            Text(Date(timeIntervalSince1970: t), style: .timer).font(font).multilineTextAlignment(.trailing).lineLimit(1)
        } else if let s = activity?.status {
            Text(s).font(font).lineLimit(1)
        }
    }

    var titles: some View {
        VStack(alignment: .leading, spacing: 2) {
            Text(activity?.title ?? "").font(.headline).lineLimit(1)
            if let body = activity?.body {
                Text(body).font(.subheadline).foregroundColor(.secondary).lineLimit(2)
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    @ViewBuilder var bar: some View {
        if let p = activity?.progress {
            ProgressBar(value: p).frame(height: 4).foregroundColor(tint)
        }
    }

    @ViewBuilder var buttons: some View {
        if let actions = activity?.actions, !actions.isEmpty {
            HStack(spacing: 8) {
                ForEach(actions.indices, id: \.self) { i in
                    HStack(spacing: 4) {
                        if let s = actions[i].symbol { Image(systemName: s) }
                        Text(actions[i].title).lineLimit(1)
                    }
                    .font(.subheadline.weight(.semibold))
                    .padding(.vertical, 8)
                    .frame(maxWidth: .infinity)
                    .background(Capsule().fill(Color.gray.opacity(0.25)))
                    .pressed(activity: id, action: actions[i].id)
                }
            }
        }
    }

    var lock: some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack(spacing: 10) {
                symbol(.title2)
                titles
                status(.title3)
            }
            bar
            buttons
        }
        .padding(16)
    }
}

@available(iOS 16.1, *)
struct ActivityLockScreen: View {
    let activity: WActivity?
    let id: String
    @Environment(\.colorScheme) var scheme

    var body: some View {
        let d = Defaults(activity: activity, id: id)
        d.place(activity?.lock) { d.lock }
            .activityBackgroundTint(activity?.bg?.color(scheme))
            .widgetURL(activity?.url.flatMap { URL(string: $0) })
    }
}
#endif
