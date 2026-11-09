// The widget extension that nsgo build puts into apps with widgets: it
// draws, with SwiftUI, the timelines the app writes with package widget,
// one directory per kind, timeline.json and its images, in the container
// of the app group they share (NSGoAppGroup in the Info.plist), or on
// macOS in a directory of the user's home (NSGoWidgetsPath), which the
// extension's sandbox lets it read. It runs none of the app's code.
// Kinds.swift, which nsgo build writes, declares the kinds of nsgo.yaml.

import ImageIO
import SwiftUI
import WidgetKit

// MARK: The timeline, as package widget writes it

final class WColor: Decodable {
    let r, g, b, a: Double
    let name: String?
    let dark: WColor?

    func color(_ scheme: ColorScheme) -> Color {
        if scheme == .dark, let dark { return dark.color(.light) }
        switch name {
        case "primary": return .primary
        case "secondary": return .secondary
        case "accent": return .accentColor
        case "clear": return .clear
        default: return Color(.sRGB, red: r, green: g, blue: b, opacity: a)
        }
    }
}

struct WFont: Decodable {
    let size: Double?
    let style: String?
    let weight: Int?
    let design: String?
    let italic: Bool?
    let digits: Bool?
    let smallCaps: Bool?

    var font: Font {
        let design: Font.Design
        switch self.design {
        case "rounded": design = .rounded
        case "serif": design = .serif
        case "monospaced": design = .monospaced
        default: design = .default
        }
        var font: Font
        if let size {
            font = .system(size: size, weight: fontWeight ?? .regular, design: design)
        } else {
            font = .system(textStyle, design: design)
            if let w = fontWeight { font = font.weight(w) }
        }
        if italic == true { font = font.italic() }
        if digits == true { font = font.monospacedDigit() }
        if smallCaps == true { font = font.smallCaps() }
        return font
    }

    var textStyle: Font.TextStyle {
        switch style {
        case "largeTitle": return .largeTitle
        case "title": return .title
        case "title2": return .title2
        case "title3": return .title3
        case "headline": return .headline
        case "subheadline": return .subheadline
        case "callout": return .callout
        case "footnote": return .footnote
        case "caption": return .caption
        case "caption2": return .caption2
        default: return .body
        }
    }

    var fontWeight: Font.Weight? {
        guard let weight else { return nil }
        switch weight {
        case ..<150: return .ultraLight
        case ..<250: return .thin
        case ..<350: return .light
        case ..<450: return .regular
        case ..<550: return .medium
        case ..<650: return .semibold
        case ..<750: return .bold
        case ..<850: return .heavy
        default: return .black
        }
    }
}

final class WNode: Decodable {
    let t: String
    let text, symbol, image: String?
    let cover: Bool?
    let date: Double?
    let dateStyle: String?
    let value: Double?
    let children: [WNode]?
    let gap: Double?
    let items, justify: String?
    let font: WFont?
    let color, bg: WColor?
    let padding: [Double]?
    let width, height: Double?
    let fillW, fillH: Bool?
    let radius: Double?
    let link: String?
    let maxLines: Int?
    let minScale: Double?
    let textAlign: String?
    let accent: Bool?
}

struct WEntry: Decodable {
    let date: Double
    let view: WNode?
    let families: [String: WNode]?
    let bg: WNode?
    let url: String?
}

struct WTimeline: Decodable {
    let v: Int
    let entries: [WEntry]
}

/// The version of timelines this extension reads.
let timelineVersion = 1

/// The directory of a kind's timeline.
func kindDirectory(_ kind: String) -> URL? {
    let info = Bundle.main.infoDictionary ?? [:]
    var dir: URL
    if let group = info["NSGoAppGroup"] as? String {
        guard let container = FileManager.default.containerURL(forSecurityApplicationGroupIdentifier: group) else { return nil }
        dir = container.appendingPathComponent("NSGoWidgets", isDirectory: true)
    } else if let path = info["NSGoWidgetsPath"] as? String, let pw = getpwuid(getuid()) {
        // The user's home, not the sandbox's, which is the container.
        dir = URL(fileURLWithPath: String(cString: pw.pointee.pw_dir), isDirectory: true).appendingPathComponent(path, isDirectory: true)
    } else {
        return nil
    }
    return dir.appendingPathComponent(kind, isDirectory: true)
}

// MARK: Timelines

struct NSGoEntry: TimelineEntry {
    let date: Date
    let entry: WEntry?
    let dir: URL?
}

struct NSGoProvider: TimelineProvider {
    let kind: String

    func load() -> [NSGoEntry] {
        guard let dir = kindDirectory(kind),
              let data = try? Data(contentsOf: dir.appendingPathComponent("timeline.json")),
              let timeline = try? JSONDecoder().decode(WTimeline.self, from: data),
              timeline.v <= timelineVersion
        else { return [] }
        return timeline.entries.map { NSGoEntry(date: Date(timeIntervalSince1970: $0.date), entry: $0, dir: dir) }
    }

    /// current is the entry showing now: the last whose date passed.
    func current() -> NSGoEntry {
        let entries = load()
        let now = Date()
        return entries.last(where: { $0.date <= now }) ?? entries.first ?? NSGoEntry(date: now, entry: nil, dir: nil)
    }

    func placeholder(in context: Context) -> NSGoEntry { current() }

    func getSnapshot(in context: Context, completion: @escaping (NSGoEntry) -> Void) {
        completion(current())
    }

    func getTimeline(in context: Context, completion: @escaping (Timeline<NSGoEntry>) -> Void) {
        let entries = load()
        // The app reloads the timeline as it writes another.
        completion(Timeline(entries: entries.isEmpty ? [NSGoEntry(date: Date(), entry: nil, dir: nil)] : entries, policy: .never))
    }
}

// MARK: Views

func familyName(_ family: WidgetFamily) -> String {
    switch family {
    case .systemSmall: return "small"
    case .systemMedium: return "medium"
    case .systemLarge: return "large"
    default:
        break
    }
    if #available(iOS 15.0, macOS 14.0, *), family == .systemExtraLarge { return "extra_large" }
    #if os(iOS)
    if #available(iOS 16.0, *) {
        switch family {
        case .accessoryCircular: return "circular"
        case .accessoryRectangular: return "rectangular"
        case .accessoryInline: return "inline"
        default: break
        }
    }
    #endif
    return ""
}

/// families returns the families of nsgo.yaml this system has.
func families(_ names: [String]) -> [WidgetFamily] {
    var out: [WidgetFamily] = []
    for name in names {
        switch name {
        case "small": out.append(.systemSmall)
        case "medium": out.append(.systemMedium)
        case "large": out.append(.systemLarge)
        case "extra_large":
            if #available(iOS 15.0, macOS 14.0, *) { out.append(.systemExtraLarge) }
        default:
            #if os(iOS)
            if #available(iOS 16.0, *) {
                switch name {
                case "circular": out.append(.accessoryCircular)
                case "rectangular": out.append(.accessoryRectangular)
                case "inline": out.append(.accessoryInline)
                default: break
                }
            }
            #endif
        }
    }
    return out
}

struct NSGoEntryView: View {
    let entry: NSGoEntry
    let name: String
    let description: String
    @Environment(\.widgetFamily) var family
    @Environment(\.colorScheme) var scheme

    var body: some View {
        content
            .widgetBackground(entry.entry?.bg.map { NodeView(node: $0, dir: entry.dir) })
            .widgetURL(entry.entry?.url.flatMap { URL(string: $0) })
    }

    @ViewBuilder var content: some View {
        if let e = entry.entry, let node = e.families?[familyName(family)] ?? e.view {
            NodeView(node: node, dir: entry.dir)
        } else {
            // The placeholder, until the app writes a timeline.
            VStack(alignment: .leading, spacing: 4) {
                Text(name).font(.headline)
                if !description.isEmpty {
                    Text(description).font(.caption).foregroundColor(.secondary)
                }
            }
            .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
        }
    }
}

extension View {
    /// widgetBackground fills the widget behind its content, past the
    /// margins the system keeps around it from iOS 17 and macOS 14, which
    /// earlier systems leave to the widget.
    @ViewBuilder func widgetBackground<B: View>(_ background: B?) -> some View {
        if #available(iOS 17.0, macOS 14.0, *) {
            if let background {
                containerBackground(for: .widget) { background }
            } else {
                containerBackground(for: .widget) { Color.clear.background(.background) }
            }
        } else {
            padding()
                .frame(maxWidth: .infinity, maxHeight: .infinity)
                .background(background)
        }
    }

    @ViewBuilder func modify<V: View>(_ when: Bool, _ fn: (Self) -> V) -> some View {
        if when { fn(self) } else { self }
    }
}

func alignment(_ s: String?) -> (HorizontalAlignment, VerticalAlignment) {
    switch s {
    case "start": return (.leading, .top)
    case "end": return (.trailing, .bottom)
    default: return (.center, .center)
    }
}

struct NodeView: View {
    let node: WNode
    let dir: URL?
    @Environment(\.colorScheme) var scheme

    var children: [WNode] { node.children ?? [] }
    var gap: CGFloat? { node.gap.map { CGFloat($0) } }

    /// frameAlignment places the content of a view larger than it: across
    /// and along its axis, which is a row's horizontal, else vertical.
    var frameAlignment: Alignment {
        let across = alignment(node.items), along = alignment(node.justify)
        if node.t == "row" { return Alignment(horizontal: along.0, vertical: across.1) }
        return Alignment(horizontal: across.0, vertical: along.1)
    }

    var body: some View {
        let radius = CGFloat(node.radius ?? 0)
        let p = node.padding ?? [0, 0, 0, 0]
        return content
            .modify(node.font != nil) { $0.font(node.font!.font) }
            .foregroundColor(node.color.map { $0.color(scheme) })
            .lineLimit(node.maxLines)
            .minimumScaleFactor(CGFloat(node.minScale ?? 1))
            .multilineTextAlignment(textAlignment)
            .padding(EdgeInsets(top: p[0], leading: p[3], bottom: p[2], trailing: p[1]))
            .frame(width: node.width.map { CGFloat($0) }, height: node.height.map { CGFloat($0) })
            .frame(maxWidth: node.fillW == true && node.width == nil ? .infinity : nil,
                   maxHeight: node.fillH == true && node.height == nil ? .infinity : nil,
                   alignment: frameAlignment)
            .background(node.bg.map { RoundedRectangle(cornerRadius: radius, style: .continuous).fill($0.color(scheme)) })
            .modify(radius > 0) { $0.clipShape(RoundedRectangle(cornerRadius: radius, style: .continuous)) }
            .modify(node.accent == true) { view in
                Group {
                    if #available(iOS 16.0, macOS 13.0, *) { view.widgetAccentable() } else { view }
                }
            }
            .linked(node.link)
    }

    var textAlignment: TextAlignment {
        switch node.textAlign {
        case "center": return .center
        case "end": return .trailing
        default: return .leading
        }
    }

    @ViewBuilder var content: some View {
        switch node.t {
        case "text":
            Text(node.text ?? "")
        case "symbol":
            Image(systemName: node.symbol ?? "questionmark")
        case "image":
            ImageNode(url: dir.flatMap { d in node.image.map { d.appendingPathComponent($0) } }, cover: node.cover == true)
        case "date":
            dateText
        case "column":
            VStack(alignment: alignment(node.items).0, spacing: gap) {
                ForEach(children.indices, id: \.self) { NodeView(node: children[$0], dir: dir) }
            }
        case "row":
            HStack(alignment: alignment(node.items).1, spacing: gap) {
                ForEach(children.indices, id: \.self) { NodeView(node: children[$0], dir: dir) }
            }
        case "stack":
            ZStack(alignment: Alignment(horizontal: alignment(node.items).0, vertical: alignment(node.items).1)) {
                ForEach(children.indices, id: \.self) { NodeView(node: children[$0], dir: dir) }
            }
        case "spacer":
            Spacer(minLength: 0)
        case "divider":
            Divider()
        case "rectangle":
            RoundedRectangle(cornerRadius: CGFloat(node.radius ?? 0), style: .continuous)
                .fill(node.color?.color(scheme) ?? .clear)
        case "progress":
            ProgressBar(value: node.value ?? 0)
                .frame(height: node.height == nil ? 4 : nil)
        case "gauge":
            ProgressRing(value: node.value ?? 0)
                .frame(width: node.width == nil && node.fillW != true ? 44 : nil,
                       height: node.height == nil && node.fillH != true ? 44 : nil)
        default:
            EmptyView()
        }
    }

    var dateText: Text {
        let date = Date(timeIntervalSince1970: node.date ?? 0)
        switch node.dateStyle {
        case "date": return Text(date, style: .date)
        case "relative": return Text(date, style: .relative)
        case "offset": return Text(date, style: .offset)
        case "timer": return Text(date, style: .timer)
        default: return Text(date, style: .time)
        }
    }
}

/// ProgressBar and ProgressRing are drawn in the foreground color, which
/// shapes fill with, as SwiftUI's progress views are not drawn the same in
/// every widget.
struct ProgressBar: View {
    let value: Double

    var body: some View {
        GeometryReader { g in
            ZStack(alignment: .leading) {
                Capsule().opacity(0.2)
                Capsule().frame(width: g.size.width * CGFloat(min(max(value, 0), 1)))
            }
        }
    }
}

struct ProgressRing: View {
    let value: Double

    var body: some View {
        GeometryReader { g in
            let line = max(3, min(g.size.width, g.size.height) * 0.12)
            ZStack {
                Circle().stroke(lineWidth: line).opacity(0.2)
                Circle()
                    .trim(from: 0, to: CGFloat(min(max(value, 0), 1)))
                    .stroke(style: StrokeStyle(lineWidth: line, lineCap: .round))
                    .rotationEffect(.degrees(-90))
            }
            .padding(line / 2)
        }
        .aspectRatio(1, contentMode: .fit)
    }
}

extension View {
    @ViewBuilder func linked(_ link: String?) -> some View {
        if let link, let url = URL(string: link) {
            Link(destination: url) { self }
        } else {
            self
        }
    }
}

/// ImageNode draws an image read at the size widgets show, which bounds
/// the memory of widgets: no larger than 1024 pixels across.
struct ImageNode: View {
    let url: URL?
    let cover: Bool

    var body: some View {
        if let image = url.flatMap(thumbnail) {
            let img = Image(decorative: image, scale: 1).resizable()
            if cover {
                Color.clear.overlay(img.scaledToFill()).clipped()
            } else {
                img.scaledToFit()
            }
        } else {
            Color.clear
        }
    }

    func thumbnail(_ url: URL) -> CGImage? {
        guard let source = CGImageSourceCreateWithURL(url as CFURL, nil) else { return nil }
        let options: [CFString: Any] = [
            kCGImageSourceCreateThumbnailFromImageAlways: true,
            kCGImageSourceCreateThumbnailWithTransform: true,
            kCGImageSourceThumbnailMaxPixelSize: 1024,
        ]
        return CGImageSourceCreateThumbnailAtIndex(source, 0, options as CFDictionary)
    }
}
