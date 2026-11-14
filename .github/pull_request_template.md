## Summary

<!-- What a user or an app author sees change, and why. Keep it to one change; link the issue it fixes (Fixes #123). -->

## Targets

<!-- Tick what this changes. A change one backend needs usually belongs in the others: make it there, or record the gap in docs/roadmap.md. -->

- [ ] Shared code (`core`, `base`, `ui`, `tsx`, the `nsgo` tool)
- [ ] macOS
- [ ] iOS
- [ ] tvOS
- [ ] watchOS
- [ ] Android
- [ ] Windows
- [ ] Linux
- [ ] Web
- [ ] Mini programs
- [ ] Docs or website only

## Screenshots

<!-- For anything visible, before and after images of the same view at its real size, with no personal data in them. Write "None" when nothing visible changes. -->

## Verification

<!-- The commands you ran and on which targets, and what you could not run. -->

- [ ] `go tool nsgo check`
- [ ] `go tool nsgo test`
- [ ] `go tool nsgo test -target <target>` on the targets above

## Checklist

- [ ] The title is a Conventional Commit, such as `fix(linux): keep the caret after a paste`; a squash merge uses it as the commit subject.
- [ ] Docs whose facts change are updated, and `CHANGELOG.md`'s `Unreleased` has a line for a capability users can see.
- [ ] New third-party code, assets and dependencies name their source and license.
