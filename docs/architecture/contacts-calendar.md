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

## Layers

`core` owns provider-neutral value types, asynchronous requests and scoped callbacks. `internal/driver` carries request IDs and serialized values. Apple uses `CNContactStore` and `EKEventStore`; Android uses `ContactsContract` and `CalendarContract` through the runtime. None of those platform records cross the driver boundary.

A request is a command in the batch, as every service is ([services](services.md#requests-and-answers)), so the platform receives it on its main thread. Pickers and editors show there, but store work runs off it: Contacts and EventKit fetches and saves on a background queue, where `CNContactStore` recommends them, and Android's `ContentResolver` calls on a worker thread, so a large address book or a wide range never freezes the UI or causes an ANR. The answer comes back through `Engine.Dispatch`, and its callback runs on the engine's goroutine under the owner that asked, or the root's when that owner is gone, as for every service. Nothing is canceled once sent: a store cannot take back a write it committed, and its answer, carrying the new ID, must still reach the app. Values are copied, and a caller cannot retain a provider cursor, URI permission or mutable platform record through the API.

## Contacts

A contact has an opaque provider ID, a display name, structured names, organization, phones, email addresses, postal addresses, birthday and image data only when the request asks for each field. The model distinguishes an absent field from an empty value and keeps labels such as “work” or “home”. The API must not assume one phone or one email is primary.

The picker returns the provider ID and the fields the user chose. A query takes a search string and a field mask, and returns a bounded page with a continuation token; it does not read the entire address book by default. A read by provider ID may fail after the user deletes or merges the contact, which is a normal not-found result.

Writing a contact is a separate operation from reading. It must say whether it creates a new record or applies a revision to an existing provider ID, and it must report conflicts instead of overwriting a newer edit silently. The first implementation may leave writes out if both platforms cannot provide the same conflict semantics; a picker and read-only API must not imply write support.

Apple requests `CNAuthorizationStatus` and uses `requestAccessForEntityType:`. Android requests `READ_CONTACTS` for reads and `WRITE_CONTACTS` only for writes. A denied or limited grant returns no contact bytes. A package declares the permissions in YAML; code does not add them to a manifest implicitly.

## Calendar

A calendar entry has an opaque calendar ID, title, account/source label, color and whether it is writable. An event has an opaque event ID, calendar ID, title, notes, location, start and end with an explicit time zone or all-day flag, recurrence, alarms and attendee display data where the provider exposes them. Date-time values are not converted to the device's current zone on the bridge.

Queries require a bounded start and end and may filter by calendar IDs. The backend returns pages or a provider-sized bounded result, never an unbounded event dump. Creating, updating and deleting an event use an explicit calendar ID and a revision token where available. An update with a stale revision fails with a conflict; deletion of an already-deleted event is idempotent.

Apple distinguishes full event access from write-only access on newer systems and uses `EKEventStore`'s access request. Android separates `READ_CALENDAR` and `WRITE_CALENDAR`. A calendar picker is optional and must be an app choice; the system permission prompt is not a substitute for asking which calendar to use.

## Privacy and capability

Contacts and calendar data use a dedicated capability, distinct from `AccessNotifications`, `AccessNetwork` and the photo picker. The final access model must decide whether contacts and calendar are separate grants. A restricted scope receives no IDs or metadata from a denied request. The backend does not log names, notes, addresses, event titles or raw provider errors containing them.

The tool adds only the declarations an app requests: Apple usage descriptions and Android permissions. Usage-description text is app-owned and localized. The service does not silently sync, cache or upload data, and it does not run in the background merely because a permission was granted.

## Verification

The implementation is complete only when these checks exist:

- headless `core` tests cover field masks, paging, answers after the owner is gone, time zones, all-day events, conflicts and denied access;
- `internal/driver` tests cover request IDs, empty versus absent fields, bounded results and large notes or payloads;
- Apple tests use fake contact and event stores for limited access, picker results, authorization changes and stale records;
- Android tests use fake `ContentResolver` providers for permission separation, cursors, account changes and revision conflicts; and
- target tests use fixture stores containing no real personal data and verify that a denied request produces no observable record metadata.

## Open decisions

These decisions need confirmation before an implementation chooses them:

1. Is the first release picker plus read-only, or does it include contact and event writes?
2. Are contacts and calendar separate capabilities and permission APIs?
3. Which contact fields, recurrence forms and attendee data are in the stable cross-platform model?
4. Does the calendar API expose provider change notifications, or require apps to requery after each operation?
5. Are provider IDs exposed as opaque strings, or wrapped in handles that expire with an engine?
