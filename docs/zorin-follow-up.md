# Zorin capture and desktop integration follow-up

## Confirmed from the reported installation

- GNOME Wayland does not expose the clipboard watch protocol used by this release.
  Having `wl-paste` installed is not evidence that capture works.
- Existing private directories were left at 0755 because `MkdirAll` does not
  change existing permissions. Store startup now restricts its private directories.
- A systemd restart request is not proof of a healthy daemon or reachable IPC.
  The remote service failure needs its journal before assigning a cause.

## Work checklist

- [x] Regression test for existing private store directory permissions.
- [x] Restrict store directories on open and set explicit systemd directory modes.
- [x] Doctor fails with actionable GNOME Wayland capture guidance.
- [x] Standalone uses Qt's desktop palette/font and normal decorations outside Omarchy.
- [x] Frontend unit suite: 43 tests; Go race/vet passed; offscreen popup loaded.
- [ ] Verify palette/font adaptation in an actual Zorin graphical session.
- [ ] Obtain failing Zorin service status and journal; reproduce offline daemon.
- [ ] Add startup-health acceptance and user-visible unsupported-session messaging.
- [x] Reject existing private directory symlinks before permission changes; regression verified.
- [ ] Correct remaining README claims about identical shortcuts, universal UI
  compatibility, package contents, i3 rules, and unchecked tarball extraction.
- [ ] Run full packaging, race/vet, desktop acceptance and independent review.
- [ ] Publish a new immutable release and verify anonymous artifact installation.

## Remote evidence needed

Run as the desktop user, not root:

```bash
systemctl --user status clipdeckd.service --no-pager -l
journalctl --user -u clipdeckd.service -b -n 100 --no-pager
```

Review logs for clipboard content or private paths before sharing.

For the currently published version, use an Xorg/X11 desktop session for Zorin
automatic capture. Do not silently use XWayland as a replacement: that would miss
clipboard changes from native Wayland applications.
