// Host.swift: the SwiftUI app an Apple Watch runs an NSGo app in, which
// nsgo links into the app's Go executable for watchOS. watchOS has no
// public UIKit or Core Animation, so the views the engine mounts are
// SwiftUI's: Go's main runs the app here with nsgo_watch_main, applies
// each commit to the Node tree through nsgo_watch_apply and draws each
// node's look and content with Core Graphics into its Canvas; this file
// places them, scrolls, edits text and shows alerts, and reports touches,
// sizes and scrolls back to Go.

import SwiftUI
import Observation
import ImageIO

// MARK: The bridge

/// What Go gives the host as nsgo_watch_connect's table, in this order.
struct Bridge {
	/// The surface's size, scale, safe area and traits.
	var size: @convention(c) (Double, Double, Double, Double, Double, Double, Double, UInt32) -> Void
	/// A touch: its phase (0 began, 1 moved, 2 ended, 3 cancelled), where
	/// it is on the surface and when, in seconds.
	var touch: @convention(c) (UInt8, Double, Double, Double) -> Void
	/// Draws node id's look and content into a context w by h, its y down.
	var draw: @convention(c) (UInt32, UnsafeMutableRawPointer, Double, Double) -> Void
	/// A scroll view's offset, and its phase (0 offset alone, 1 dragging,
	/// 2 gliding, 3 at rest, 4 animating).
	var scroll: @convention(c) (UInt32, Double, Double, UInt8) -> Void
	/// A display frame began, at seconds.
	var frame: @convention(c) (Double) -> Void
	/// A field's text changed (0), was submitted (1), took the focus (2)
	/// or lost it (3).
	var text: @convention(c) (UInt32, UInt8, UnsafePointer<UInt8>?, Int) -> Void
	/// Alert id was answered with the action at index.
	var alert: @convention(c) (UInt32, Int32) -> Void
	/// The app became active (0), inactive (1) or went to the background (2).
	var lifecycle: @convention(c) (UInt8) -> Void
	/// The PNG nsgo_watch_snapshot rendered.
	var snapshot: @convention(c) (UnsafePointer<UInt8>?, Int) -> Void
}

var bridge: Bridge?

/// nsgo_watch_connect takes the bridge: n function pointers in Bridge's
/// order.
@_cdecl("nsgo_watch_connect")
public func nsgoWatchConnect(_ fns: UnsafePointer<UnsafeRawPointer>, _ n: Int) {
	guard n >= 9 else { return }
	bridge = Bridge(
		size: unsafeBitCast(fns[0], to: (@convention(c) (Double, Double, Double, Double, Double, Double, Double, UInt32) -> Void).self),
		touch: unsafeBitCast(fns[1], to: (@convention(c) (UInt8, Double, Double, Double) -> Void).self),
		draw: unsafeBitCast(fns[2], to: (@convention(c) (UInt32, UnsafeMutableRawPointer, Double, Double) -> Void).self),
		scroll: unsafeBitCast(fns[3], to: (@convention(c) (UInt32, Double, Double, UInt8) -> Void).self),
		frame: unsafeBitCast(fns[4], to: (@convention(c) (Double) -> Void).self),
		text: unsafeBitCast(fns[5], to: (@convention(c) (UInt32, UInt8, UnsafePointer<UInt8>?, Int) -> Void).self),
		alert: unsafeBitCast(fns[6], to: (@convention(c) (UInt32, Int32) -> Void).self),
		lifecycle: unsafeBitCast(fns[7], to: (@convention(c) (UInt8) -> Void).self),
		snapshot: unsafeBitCast(fns[8], to: (@convention(c) (UnsafePointer<UInt8>?, Int) -> Void).self))
}

/// nsgo_watch_main runs the app on the calling thread, the main thread,
/// once Go connected the bridge. It does not return.
@_cdecl("nsgo_watch_main")
public func nsgoWatchMain() {
	NSGoWatchApp.main()
}

/// nsgo_watch_apply applies a batch of ops Go wrote (Store.apply), on the
/// main thread.
@_cdecl("nsgo_watch_apply")
public func nsgoWatchApply(_ data: UnsafePointer<UInt8>, _ n: Int) {
	Store.shared.apply(Reader(base: data, count: n))
}

/// nsgo_watch_snapshot renders the surface and hands Go the PNG, for
/// tests' screenshots.
@MainActor @_cdecl("nsgo_watch_snapshot")
public func nsgoWatchSnapshot() {
	let store = Store.shared
	let renderer = ImageRenderer(content: ZStack(alignment: .topLeading) {
		Color.black
		if let root = store.root { NodeView(node: root) }
	}
	.frame(width: store.size.width, height: store.size.height, alignment: .topLeading)
	.environment(\.snapshotting, true))
	renderer.scale = store.scale
	guard let image = renderer.cgImage else { return }
	let data = NSMutableData()
	guard let dest = CGImageDestinationCreateWithData(data, "public.png" as CFString, 1, nil) else { return }
	CGImageDestinationAddImage(dest, image, nil)
	guard CGImageDestinationFinalize(dest) else { return }
	bridge?.snapshot(data.bytes.assumingMemoryBound(to: UInt8.self), data.length)
}

/// nsgo_watch_tick starts the display frames (on 1) or stops them.
@_cdecl("nsgo_watch_tick")
public func nsgoWatchTick(_ on: UInt8) {
	Store.shared.ticking = on != 0
}

func sendText(_ id: UInt32, _ kind: UInt8, _ s: String) {
	var s = s
	s.withUTF8 { b in bridge?.text(id, kind, b.baseAddress, b.count) }
}

// MARK: The tree

/// Reader reads what Go's writer wrote: little-endian numbers and
/// length-prefixed UTF-8.
struct Reader {
	let base: UnsafePointer<UInt8>
	let count: Int
	var at = 0

	var more: Bool { at < count }

	mutating func u8() -> UInt8 {
		defer { at += 1 }
		return base[at]
	}

	mutating func u32() -> UInt32 {
		var v: UInt32 = 0
		memcpy(&v, base + at, 4)
		at += 4
		return UInt32(littleEndian: v)
	}

	mutating func f64() -> Double {
		var v: UInt64 = 0
		memcpy(&v, base + at, 8)
		at += 8
		return Double(bitPattern: UInt64(littleEndian: v))
	}

	mutating func rect() -> CGRect { CGRect(x: f64(), y: f64(), width: f64(), height: f64()) }

	mutating func string() -> String {
		let n = Int(u32())
		defer { at += n }
		return String(decoding: UnsafeBufferPointer(start: base + at, count: n), as: UTF8.self)
	}
}

/// The kinds of node Go makes, as driver.ViewKind numbers them.
enum Kind: UInt8 {
	case view = 1, text, input, scroll, image, modal, canvas
}

/// Node is one view the engine mounted.
@Observable final class Node: Identifiable {
	let id: UInt32
	let kind: Kind
	var children: [Node] = []
	@ObservationIgnored weak var parent: Node?

	var frame = CGRect.zero
	var opacity = 1.0
	var hidden = false
	/// clips says the children show only inside the box, rounded by radii
	/// (top left, top right, bottom right, bottom left).
	var clips = false
	var radii = RectangleCornerRadii()
	/// transform moves the node from its own top left corner.
	var transform = CGAffineTransform.identity
	/// draws says Go draws the node's look into ink, a rectangle in the
	/// node's own coordinates; version changes each time it draws again.
	var draws = false
	/// culled says the node is too far from what shows to be made.
	var culled = false
	var ink = CGRect.zero
	var version = 0

	// A scroll view's.
	var horizontal = false
	var indicator = true
	var content = CGSize.zero
	var locked = false
	var position = ScrollPosition()
	/// offset is where the scroll view last reported it was.
	@ObservationIgnored var offset = CGPoint.zero

	// A text field's.
	var text = ""
	var placeholder = ""
	var secure = false
	var disabled = false
	var focused = false

	init(id: UInt32, kind: Kind) {
		self.id = id
		self.kind = kind
	}
}

/// An alert the app shows.
struct Alert: Identifiable {
	let id: UInt32
	let title: String
	let message: String
	let actions: [(String, UInt8)]
}

@Observable final class Store {
	static let shared = Store()

	@ObservationIgnored var nodes: [UInt32: Node] = [:]
	var root: Node?
	var ticking = false
	var alert: Alert?
	/// The surface's size and pixels a point, for snapshots.
	@ObservationIgnored var size = CGSize.zero
	@ObservationIgnored var scale = 2.0

	/// The ops Go writes, as watchos.go's op constants number them.
	enum Op: UInt8 {
		case create = 1, insert, remove, delete, frame, look, ink, redraw, scroll, scrollTo, input, focus, blur,
			alert, root, lock, dismiss, cull
	}

	func apply(_ r: Reader) {
		var r = r
		while r.more {
			guard let op = Op(rawValue: r.u8()) else { return }
			switch op {
			case .create:
				let id = r.u32()
				nodes[id] = Node(id: id, kind: Kind(rawValue: r.u8()) ?? .view)
			case .insert:
				let id = r.u32(), parent = r.u32(), before = r.u32()
				guard let n = nodes[id], let p = nodes[parent] else { continue }
				detach(n)
				// Before is the sibling n goes under; 0 puts it on top.
				if before != 0, let i = p.children.firstIndex(where: { $0.id == before }) {
					p.children.insert(n, at: i)
				} else {
					p.children.append(n)
				}
				n.parent = p
			case .remove:
				if let n = nodes[r.u32()] { detach(n) }
			case .delete:
				let id = r.u32()
				if let n = nodes.removeValue(forKey: id) { detach(n) }
			case .frame:
				let id = r.u32(), f = r.rect()
				if let n = nodes[id], n.frame != f { n.frame = f }
			case .look:
				let id = r.u32()
				let opacity = r.f64(), hidden = r.u8() != 0, clips = r.u8() != 0
				let radii = RectangleCornerRadii(topLeading: r.f64(), bottomLeading: r.f64(), bottomTrailing: r.f64(), topTrailing: r.f64())
				let t = CGAffineTransform(a: r.f64(), b: r.f64(), c: r.f64(), d: r.f64(), tx: r.f64(), ty: r.f64())
				guard let n = nodes[id] else { continue }
				if n.opacity != opacity { n.opacity = opacity }
				if n.hidden != hidden { n.hidden = hidden }
				if n.clips != clips { n.clips = clips }
				if n.radii != radii { n.radii = radii }
				if n.transform != t { n.transform = t }
			case .ink:
				let id = r.u32(), draws = r.u8() != 0, ink = r.rect()
				guard let n = nodes[id] else { continue }
				if n.draws != draws { n.draws = draws }
				if n.ink != ink { n.ink = ink }
				n.version += 1
			case .redraw:
				nodes[r.u32()]?.version += 1
			case .scroll:
				let id = r.u32(), horizontal = r.u8() != 0, indicator = r.u8() != 0
				let content = CGSize(width: r.f64(), height: r.f64())
				guard let n = nodes[id] else { continue }
				if n.horizontal != horizontal { n.horizontal = horizontal }
				if n.indicator != indicator { n.indicator = indicator }
				if n.content != content { n.content = content }
			case .scrollTo:
				let id = r.u32(), x = r.f64(), y = r.f64(), animated = r.u8() != 0
				guard let n = nodes[id] else { continue }
				if animated {
					withAnimation { n.position.scrollTo(point: CGPoint(x: x, y: y)) }
				} else {
					n.position.scrollTo(point: CGPoint(x: x, y: y))
				}
			case .input:
				let id = r.u32(), placeholder = r.string(), secure = r.u8() != 0, disabled = r.u8() != 0
				guard let n = nodes[id] else { continue }
				n.placeholder = placeholder
				n.secure = secure
				n.disabled = disabled
				let text = r.string()
				if n.text != text { n.text = text }
			case .focus:
				nodes[r.u32()]?.focused = true
			case .blur:
				nodes[r.u32()]?.focused = false
			case .alert:
				let id = r.u32(), title = r.string(), message = r.string()
				var actions: [(String, UInt8)] = []
				for _ in 0..<r.u8() { actions.append((r.string(), r.u8())) }
				alert = Alert(id: id, title: title, message: message, actions: actions)
			case .root:
				let n = nodes[r.u32()]
				if root !== n { root = n }
			case .cull:
				let id = r.u32(), culled = r.u8() != 0
				if let n = nodes[id], n.culled != culled { n.culled = culled }
			case .dismiss:
				if alert?.id == r.u32() { alert = nil }
			case .lock:
				let id = r.u32(), locked = r.u8() != 0
				if let n = nodes[id], n.locked != locked { n.locked = locked }
			}
		}
	}

	func detach(_ n: Node) {
		if let p = n.parent {
			p.children.removeAll { $0 === n }
			n.parent = nil
		}
	}
}

// MARK: The views

extension EnvironmentValues {
	/// snapshotting is set for what nsgo_watch_snapshot renders.
	@Entry var snapshotting = false
}

/// NodeView shows a node at its frame in its parent: its look under its
/// children, which it clips when it clips. A leaf is its look alone, so
/// most nodes cost SwiftUI a Canvas and their placement.
struct NodeView: View {
	let node: Node

	var body: some View {
		content
			.opacity(node.hidden ? 0 : node.opacity)
			.allowsHitTesting(!node.hidden)
			.transformEffect(node.transform)
			.offset(x: node.frame.minX, y: node.frame.minY)
	}

	@ViewBuilder var content: some View {
		switch node.kind {
		case .scroll:
			ZStack(alignment: .topLeading) {
				look
				ScrollNode(node: node)
			}
			.frame(width: node.frame.width, height: node.frame.height, alignment: .topLeading)
		case .input:
			ZStack(alignment: .topLeading) {
				look
				FieldNode(node: node)
			}
			.frame(width: node.frame.width, height: node.frame.height, alignment: .topLeading)
		default:
			if node.children.isEmpty {
				look.frame(width: node.frame.width, height: node.frame.height, alignment: .topLeading)
			} else {
				ZStack(alignment: .topLeading) {
					look
					Children(node: node)
				}
				.frame(width: node.frame.width, height: node.frame.height, alignment: .topLeading)
			}
		}
	}

	@ViewBuilder var look: some View {
		if node.draws {
			Drawn(id: node.id, version: node.version)
				.frame(width: node.ink.width, height: node.ink.height)
				.offset(x: node.ink.minX, y: node.ink.minY)
				.allowsHitTesting(false)
		}
	}
}

/// Children are a node's children, each placed at its frame: in the
/// node's own stack, or in one clipped to its box when it clips.
struct Children: View {
	let node: Node

	var body: some View {
		if node.clips {
			ZStack(alignment: .topLeading) {
				ForEach(node.children.filter { !$0.culled }) { NodeView(node: $0) }
			}
			.frame(width: node.frame.width, height: node.frame.height, alignment: .topLeading)
			.clipShape(UnevenRoundedRectangle(cornerRadii: node.radii))
		} else {
			ForEach(node.children.filter { !$0.culled }) { NodeView(node: $0) }
		}
	}
}

/// Drawn is the Canvas Go draws a node's look into; a new version draws it
/// again.
struct Drawn: View {
	let id: UInt32
	let version: Int

	var body: some View {
		Canvas { ctx, size in
			ctx.withCGContext { cg in
				bridge?.draw(id, Unmanaged.passUnretained(cg).toOpaque(), size.width, size.height)
			}
		}
	}
}

/// ScrollNode scrolls a scroll view's children, which Go placed in its
/// content, and reports where it is.
struct ScrollNode: View {
	@Bindable var node: Node
	@Environment(\.snapshotting) private var snapshotting

	@ViewBuilder var body: some View {
		if snapshotting {
			// ImageRenderer draws no ScrollView: the content where it is.
			ZStack(alignment: .topLeading) {
				ForEach(node.children.filter { !$0.culled }) { NodeView(node: $0) }
			}
			.offset(x: -node.offset.x, y: -node.offset.y)
			.frame(width: node.frame.width, height: node.frame.height, alignment: .topLeading)
			.clipped()
		} else {
			scroller
		}
	}

	@ViewBuilder var scroller: some View {
		let size = CGSize(width: node.horizontal ? max(node.content.width, node.frame.width) : node.frame.width,
			height: node.horizontal ? node.frame.height : max(node.content.height, node.frame.height))
		ScrollView(node.horizontal ? .horizontal : .vertical, showsIndicators: node.indicator) {
			ZStack(alignment: .topLeading) {
				ForEach(node.children.filter { !$0.culled }) { NodeView(node: $0) }
			}
			.frame(width: size.width, height: size.height, alignment: .topLeading)
		}
		.scrollPosition($node.position)
		.scrollDisabled(node.locked)
		.contentMargins(0, for: .scrollContent)
		.onScrollGeometryChange(for: CGPoint.self) { $0.contentOffset } action: { _, at in
			node.offset = at
			bridge?.scroll(node.id, at.x, at.y, 0)
		}
		.onScrollPhaseChange { _, phase in
			let code: UInt8 = switch phase {
			case .interacting, .tracking: 1
			case .decelerating: 2
			case .animating: 4
			default: 3
			}
			bridge?.scroll(node.id, .nan, .nan, code)
		}
		.frame(width: node.frame.width, height: node.frame.height)
		.clipShape(UnevenRoundedRectangle(cornerRadii: node.radii))
	}
}

/// FieldNode is a text field: Go draws its text, and a TextField all but
/// transparent over it takes the taps that open the system's text input.
struct FieldNode: View {
	@Bindable var node: Node
	@FocusState private var focus: Bool

	var body: some View {
		ZStack(alignment: .topLeading) {
			Group {
				if node.secure {
					SecureField(node.placeholder, text: $node.text)
				} else {
					TextField(node.placeholder, text: $node.text)
				}
			}
			.focused($focus)
			.disabled(node.disabled)
			.opacity(0.02)
			.frame(width: node.frame.width, height: node.frame.height)
			.onChange(of: node.text) { _, text in sendText(node.id, 0, text) }
			.onSubmit { sendText(node.id, 1, node.text) }
			.onChange(of: focus) { _, on in
				node.focused = on
				sendText(node.id, on ? 2 : 3, node.text)
			}
			.onChange(of: node.focused) { _, on in
				if focus != on { focus = on }
			}
			Children(node: node)
		}
	}
}

/// Ticker reports a display frame while Go asks for them.
struct Ticker: View {
	var body: some View {
		TimelineView(.animation) { ctx in
			Color.clear.onChange(of: ctx.date) { _, date in
				bridge?.frame(date.timeIntervalSinceReferenceDate)
			}
		}
	}
}

/// Root is the surface: the root node over the whole screen, the touches
/// on it and what it reports of its size and traits.
struct Root: View {
	@State private var store = Store.shared
	@Environment(\.displayScale) private var scale
	@Environment(\.colorScheme) private var scheme
	@Environment(\.accessibilityReduceMotion) private var reduceMotion
	@Environment(\.accessibilityVoiceOverEnabled) private var voiceOver
	@Environment(\.scenePhase) private var phase
	@Environment(\.dynamicTypeSize) private var typeSize
	@State private var touching = false

	var body: some View {
		GeometryReader { geo in
			ZStack(alignment: .topLeading) {
				if store.ticking { Ticker().frame(width: 0, height: 0) }
				if let root = store.root { NodeView(node: root) }
			}
			.frame(width: geo.size.width, height: geo.size.height, alignment: .topLeading)
			.contentShape(Rectangle())
			.simultaneousGesture(
				DragGesture(minimumDistance: 0, coordinateSpace: .named("nsgo"))
					.onChanged { g in
						bridge?.touch(touching ? 1 : 0, g.location.x, g.location.y, g.time.timeIntervalSinceReferenceDate)
						touching = true
					}
					.onEnded { g in
						touching = false
						bridge?.touch(2, g.location.x, g.location.y, g.time.timeIntervalSinceReferenceDate)
					})
			.onAppear { report(geo) }
			.onChange(of: geo.size) { report(geo) }
			.onChange(of: scheme) { report(geo) }
			.onChange(of: reduceMotion) { report(geo) }
			.onChange(of: voiceOver) { report(geo) }
			.onChange(of: typeSize) { report(geo) }
		}
		.coordinateSpace(.named("nsgo"))
		.ignoresSafeArea()
		.alert(store.alert?.title ?? "", isPresented: Binding(get: { store.alert != nil }, set: { if !$0 { store.alert = nil } }),
			presenting: store.alert) { a in
			ForEach(Array(a.actions.enumerated()), id: \.offset) { i, action in
				Button(action.0, role: action.1 == 2 ? .destructive : action.1 == 1 ? .cancel : nil) {
					bridge?.alert(a.id, Int32(i))
				}
			}
		} message: { a in
			if !a.message.isEmpty { Text(a.message) }
		}
		.onChange(of: phase, initial: true) { _, p in
			bridge?.lifecycle(p == .active ? 0 : p == .inactive ? 1 : 2)
		}
	}

	/// report sends the surface's size, scale, safe area and traits: dark
	/// (1), reduce motion (2) and VoiceOver (4), with the text size's
	/// step from the default in the bits above 8.
	func report(_ geo: GeometryProxy) {
		store.size = geo.size
		store.scale = scale
		let insets = geo.safeAreaInsets
		var flags: UInt32 = 0
		if scheme == .dark { flags |= 1 }
		if reduceMotion { flags |= 2 }
		if voiceOver { flags |= 4 }
		let steps: [DynamicTypeSize] = [.xSmall, .small, .medium, .large, .xLarge, .xxLarge, .xxxLarge,
			.accessibility1, .accessibility2, .accessibility3, .accessibility4, .accessibility5]
		let step = (steps.firstIndex(of: typeSize) ?? 3) - 3
		flags |= UInt32(bitPattern: Int32(step + 128)) << 8
		bridge?.size(geo.size.width, geo.size.height, scale, insets.top, insets.leading, insets.bottom, insets.trailing, flags)
	}
}

struct NSGoWatchApp: App {
	var body: some Scene {
		WindowGroup { Root() }
	}
}
