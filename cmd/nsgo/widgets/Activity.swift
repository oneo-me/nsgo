// What the widget extension and the app's NSGoWidgetKit share of Live
// Activities: their attributes, whose content-state is the JSON package
// widget writes, and the intent a button of one runs, which iOS runs in
// the app's process, where NSGO_APP is set (Activities.swift).

#if os(iOS)
import ActivityKit
import AppIntents
import Foundation

/// JSONValue holds a JSON value as it decodes, so a content-state is the
/// JSON package widget writes and a push carries.
enum JSONValue: Codable, Hashable {
    case null
    case bool(Bool)
    case number(Double)
    case string(String)
    case array([JSONValue])
    case object([String: JSONValue])

    init(from decoder: Decoder) throws {
        let c = try decoder.singleValueContainer()
        if c.decodeNil() {
            self = .null
        } else if let b = try? c.decode(Bool.self) {
            self = .bool(b)
        } else if let n = try? c.decode(Double.self) {
            self = .number(n)
        } else if let s = try? c.decode(String.self) {
            self = .string(s)
        } else if let a = try? c.decode([JSONValue].self) {
            self = .array(a)
        } else {
            self = .object(try c.decode([String: JSONValue].self))
        }
    }

    func encode(to encoder: Encoder) throws {
        var c = encoder.singleValueContainer()
        switch self {
        case .null: try c.encodeNil()
        case let .bool(b): try c.encode(b)
        case let .number(n): try c.encode(n)
        case let .string(s): try c.encode(s)
        case let .array(a): try c.encode(a)
        case let .object(o): try c.encode(o)
        }
    }

    subscript(key: String) -> JSONValue? {
        if case let .object(o) = self { return o[key] }
        return nil
    }

    var number: Double? {
        if case let .number(n) = self { return n }
        return nil
    }
}

@available(iOS 16.1, *)
struct NSGoActivityAttributes: ActivityAttributes {
    struct ContentState: Codable, Hashable {
        var value: JSONValue

        init(value: JSONValue) { self.value = value }
        init(from decoder: Decoder) throws { value = try JSONValue(from: decoder) }
        func encode(to encoder: Encoder) throws { try value.encode(to: encoder) }

        /// stale is when the content goes out of date.
        var stale: Date? { value["stale"]?.number.map { Date(timeIntervalSince1970: $0) } }
    }

    /// kind is what the app started the activity as, and id names it.
    var kind: String
    var id: String
}

/// NSGoActionIntent is a press of a button of an activity: its action, as
/// package widget's Button names it.
@available(iOS 17.0, *)
struct NSGoActionIntent: LiveActivityIntent {
    static var title: LocalizedStringResource = "Action"
    static var isDiscoverable: Bool = false

    @Parameter(title: "Activity") var activity: String
    @Parameter(title: "Action") var action: String

    init() {}

    init(activity: String, action: String) {
        self.activity = activity
        self.action = action
    }

    func perform() async throws -> some IntentResult {
        #if NSGO_APP
        await Presses.press(activity: activity, action: action)
        #endif
        return .result()
    }
}
#endif
