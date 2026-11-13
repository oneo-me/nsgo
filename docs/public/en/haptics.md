---
title: Haptics
description: Play feedback under the user's finger as a value changes, elements collide or a task ends.
group: Platform features
order: 46
---

## Playing one

`mobile.PlayHaptic` plays feedback under the user's finger on iOS and Android, and does nothing elsewhere, so pair it with what the user sees. `HapticSelection` marks a value changing, a picker's, a segment's or a slider's step; `HapticLight` to `HapticRigid` mark elements colliding, by their weight and give; and `HapticSuccess`, `HapticWarning` and `HapticError` mark a task ending so.

```go
mobile.PlayHaptic(cx, mobile.HapticSelection)
```

## When

Play it as the gesture happens, not as the answer arrives: a selection as a value passes, an impact as a view lands, an outcome as a task ends. Play one for a change the user caused, and none for what the app did on its own.

## Next

- [Notifications](notifications.md): the notifications and badges that carry the same news.
- [Platform services](services.md): the rest of what the system offers.
- [Accessibility](accessibility.md): the announcement a user who feels nothing hears.
