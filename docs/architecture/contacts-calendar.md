# Contacts and calendar

This is the design for issue [#57](https://github.com/nsgo-dev/nsgo/issues/57). It is not an implemented API. Contacts and calendar are separate services because their stores, permissions and user expectations differ, even though both are personal data on a phone.

## Scope

The first design covers:

- choosing a contact through the platform picker;
- reading a contact's selected fields and, where permission allows, querying contacts;
- listing calendars and events in a time range;
- creating, updating and deleting an event with an explicit user action; and
- reporting permission, unavailable-store and provider errors without exposing platform objects.

Reminders, accounts, contact groups, event attachments and continuous background synchronization are separate until their data and lifecycle rules are defined. NSGo never becomes the source of truth: an app owns the server or local database it synchronizes with.

## Targets

| Target | Contact picker | Contacts | Event editor | Calendar |
| --- | --- | --- | --- | --- |
| iOS | `CNContactPickerViewController` | `CNContactStore` | `EKEventEditViewController` | `EKEventStore` |
| macOS | `CNContactPicker` | `CNContactStore` | None: the SDK has no EventKitUI | `EKEventStore` |
| Android | `ACTION_PICK` on `ContactsContract.Contacts` | `ContactsContract` | `ACTION_INSERT` on `CalendarContract.Events` | `CalendarContract` |

tvOS has neither Contacts nor EventKit, and watchOS has both but neither ContactsUI nor EventKitUI; this design leaves both out, with Windows, Linux, the web and mini programs. `internal/platform/apple` serves tvOS and watchOS too, so it loads these frameworks only on iOS and macOS. `Env` flags, as `Env.CanTakePhoto` does, say where each of the four columns exists; elsewhere a request answers unavailable without asking, which an app can tell from denied.

## Layers

`core` owns provider-neutral value types, asynchronous requests and scoped callbacks. `internal/driver` carries request IDs and serialized values. Platform records, cursors and URIs never cross the driver boundary.

A request is a command in the batch, as every service is ([services](services.md#requests-and-answers)), so the platform receives it on its main thread. Pickers and editors show there, but store work runs off it: Contacts and EventKit fetches and saves on a background queue, where `CNContactStore` recommends them, and Android's `ContentResolver` calls on a worker thread, so a large address book or a wide range never freezes the UI or causes an ANR. The answer comes back through `Engine.Dispatch`, and its callback runs on the engine's goroutine under the owner that asked, or the root's when that owner is gone, as for every service. Nothing is canceled once sent: a store cannot take back a write it committed, and its answer, carrying the new ID, must still reach the app. Values are copied, and a caller cannot retain a provider cursor, URI permission or mutable platform record through the API.

## Contacts

A contact has an opaque provider ID, a display name, structured names, organization, phones, email addresses, postal addresses, birthday and image data only when the request asks for each field. The model distinguishes an absent field from an empty value and keeps labels such as “work” or “home”. The API must not assume one phone or one email is primary.

The provider ID is `CNContact.identifier` on Apple and, on Android, the `LOOKUP_KEY` and `_ID` pair `Contacts.getLookupUri` joins, because `_ID` alone changes when the provider joins or splits contacts. Apple's unified contacts may come back under another identifier too, so a read by provider ID may fail after the user or the provider deletes, merges or splits the contact, which is a normal not-found result.

The picker returns the provider ID and the fields the user chose. A query takes a search string and a field mask, and returns a bounded page with a continuation token; it does not read the entire address book by default.

Writing a contact is a separate operation from reading. It must say whether it creates a new record or applies a revision to an existing provider ID, and it must report conflicts instead of overwriting a newer edit silently. The first implementation may leave writes out if both platforms cannot provide the same conflict semantics; a picker and read-only API must not imply write support.

The permission reads as not asked, granted, limited, denied or restricted. Apple reads it with `CNContactStore`'s `authorizationStatusForEntityType:` and asks with `requestAccessForEntityType:completionHandler:`; Android asks for `READ_CONTACTS` for reads and `WRITE_CONTACTS` only for writes. Limited is iOS 18's `CNAuthorizationStatusLimited`: queries and reads see only the contacts the user shared, and one outside them is not found, not an error. Denied and restricted return no contact bytes. A package declares the usage description and permissions in YAML; code does not add them to a manifest implicitly.

## Calendar

A calendar entry has an opaque calendar ID, title, account/source label, color and whether it is writable. An event has an opaque event ID, calendar ID, title, notes, location, start and end with an explicit time zone or all-day flag, recurrence, alarms and attendee display data where the provider exposes them. Date-time values are not converted to the device's current zone on the bridge.

Queries require a bounded start and end and may filter by calendar IDs. The backend returns pages or a provider-sized bounded result, never an unbounded event dump. A query returns occurrences, not series: each carries its event ID, its original start (EventKit's `occurrenceDate`, Android's `Instances.BEGIN`) and whether it was detached from its series. Android queries `CalendarContract.Instances`, whose `_ID` is not an event's, and returns `Instances.EVENT_ID`.

Creating an event takes an explicit calendar ID; updating and deleting take the event ID and a revision token where available. An update with a stale revision fails with a conflict. A change to a recurring event names the occurrence and a span, this occurrence or this and the following ones, as EventKit's `EKSpanThisEvent` and `EKSpanFutureEvents`; Android changes one occurrence through an exception on `Events.CONTENT_EXCEPTION_URI`, and the following ones by ending the series' rule and inserting a new series. A change to a recurring event without a span fails rather than changing the whole series.

An event ID does not survive every change: EventKit's `eventIdentifier` will likely change when the event moves to another calendar, on this device or through a sync from another. An update or delete whose ID no longer resolves fails with not-found, never reports success, so the app can query again rather than believe the event is gone.

The permission reads as not asked, write-only, full, denied or restricted. From iOS 17 and macOS 14, Apple asks with `requestWriteOnlyAccessToEventsWithCompletion:` or `requestFullAccessToEventsWithCompletion:` and needs `NSCalendarsWriteOnlyAccessUsageDescription` or `NSCalendarsFullAccessUsageDescription`; on iOS 17 an app with only `NSCalendarsUsageDescription` is denied without a prompt. Below them, `requestAccessToEntityType:` asks for the one level there is, with `NSCalendarsUsageDescription`, so an app that supports both declares both keys.

Write-only access lets an app create events and nothing else: listing calendars returns one virtual calendar that is no real one, a query returns nothing, not even the events the app created, and the user chooses the calendar a new event goes to. Under it the calendar list holds that one entry, marked as the system's choice, creating into it saves where the user chooses, and queries, updates and deletes fail with a permission error rather than answering empty or not-found. Android has no write-only level: choosing a calendar reads `CalendarContract.Calendars`, which needs `READ_CALENDAR`, so creating, updating or deleting needs `READ_CALENDAR` and `WRITE_CALENDAR` together.

A calendar picker is optional and must be an app choice; the system permission prompt is not a substitute for asking which calendar to use.

## Privacy and capability

Contacts and calendar data use a dedicated capability, distinct from `AccessNotifications`, `AccessNetwork` and the photo picker. The final access model must decide whether contacts and calendar are separate grants. A restricted scope receives no IDs or metadata from a denied request. The backend does not log names, notes, addresses, event titles or raw provider errors containing them.

The tool adds the declarations an app asks for in `nsgo.yaml`: Apple usage descriptions from `apple.info` and Android permissions from `android.permissions`. On macOS it also claims `com.apple.security.personal-information.addressbook` when `apple.info` has `NSContactsUsageDescription`, and `com.apple.security.personal-information.calendars` when it has a calendar usage description, as `hardenedEntitlements` in `cmd/nsgo/sign.go` does for the microphone and the camera: the hardened runtime of a notarized build and the App Sandbox of a store build deny Contacts and EventKit without them. Usage-description text is app-owned and localized. The service does not silently sync, cache or upload data, and it does not run in the background merely because a permission was granted.

## Verification

The implementation is complete only when these checks exist:

- headless `core` tests cover field masks, paging, answers after the owner is gone, time zones, all-day events, occurrences and spans, conflicts, not-found IDs, limited, write-only and denied access, and unavailable targets;
- `internal/driver` tests cover request IDs, empty versus absent fields, bounded results and large notes or payloads;
- Apple tests use fake contact and event stores for limited and write-only access, picker results, authorization changes, stale records and changed identifiers;
- Android tests use fake `ContentResolver` providers for permission separation, cursors, `Instances` versus `Events` IDs, lookup keys, account changes and revision conflicts;
- `cmd/nsgo` tests check the macOS entitlements each usage description brings; and
- target tests use fixture stores containing no real personal data and verify that a denied request produces no observable record metadata.

## Open decisions

These decisions need confirmation before an implementation chooses them:

1. Is the first release picker plus read-only, or does it include contact and event writes?
2. Are contacts and calendar separate capabilities and permission APIs?
3. Which contact fields, recurrence forms and attendee data are in the stable cross-platform model?
4. Does the calendar API expose provider change notifications, or require apps to requery after each operation?
5. Are provider IDs exposed as opaque strings, or wrapped in handles that expire with an engine?
