// Package pkg signs TSX apps into packages and verifies them, for code
// an app downloads: its own screens updated, or plugins. It ports NSH's
// nsh-package; a host runs what Trust.Verify returns in tsx.View.
//
// # Keys
//
// A root key stays offline, and apps trust its public key (several roots
// may be trusted, which is how roots rotate). A root certifies release
// keys (Certify): a Certificate names the release key, the time it may
// sign in and the app IDs it may sign for ("*", "com.example.*" or an
// ID). Packages are signed by a release key and carry its certificate,
// so release keys rotate without updating the apps. A key's ID is the
// first 8 bytes of its public key's SHA-256, in hex. Every signature is
// over the context, a zero byte and the SHA-256 of the message, with a
// context for packages, certificates and revocation lists each, so one
// never passes for another.
//
// # Revocation and rollback
//
// A root also signs revocation lists (Revoke): release keys and packages,
// by their SHA-256, that must not load any more. Each has a sequence
// number, and a Trust takes only a list newer than its own, so an old
// list cannot be replayed. A Trust also keeps the highest build it
// accepted of each app and refuses older ones. With a directory
// (OpenTrust) both survive restarts.
//
// # Format
//
// Integers are little-endian.
//
//	offset  size  field
//	0       4     magic "NSGP"
//	4       2     format version (1)
//	6       2     flags (0)
//	8       4     H: the header's length
//	12      H     the header, UTF-8 JSON
//	12+H    B     the files' bytes, in the header's order
//	12+H+B  64    the release key's Ed25519 signature of all before it
//
// A package's identity, which revocation lists name, is the SHA-256 of
// the whole file. A package holds the bundle's source, whose assets and
// source map are inline; goja runs source alone, which it parses safely,
// so a package is trusted for where it came from, never for memory
// safety, as NSH's bytecode is.
//
// # Format evolution
//
// Packages outlive the runtime that reads them, so what a runtime does not
// know never widens what a package may do:
//
//   - header and app fields it does not know are ignored;
//   - a file of a kind it does not know, or with a field it does not know,
//     is not used, though it still tiles the data and is hashed;
//   - a header field it must not ignore is named in the header's
//     "critical" list, and a runtime that does not know a field listed
//     there refuses the package.
//
// What was ignored is in Info.Ignored. An incompatible change bumps the
// format version, which runtimes require to match. Certificates and
// revocation lists refuse fields they do not know.
package pkg
