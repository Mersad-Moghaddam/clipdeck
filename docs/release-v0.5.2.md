# ClipDeck v0.5.2

## Privacy, diagnostics and desktop appearance

- Restrict existing history, blob and thumbnail directories to `0700` on store
  startup. Reject symlinks at these private directory paths before changing modes.
- Set explicit systemd runtime/state/configuration directory modes to `0700`.
- Report GNOME Wayland capture as an unsupported required check in Doctor and
  show a persistent warning in the standalone popup.
- Use Qt's desktop palette and font, and normal window decorations, outside
  Omarchy. Palette integration depends on the installed Qt platform theme;
  custom GTK themes are not guaranteed to transfer exactly.
- Preserve the existing Omarchy shell overlay unchanged.

## Zorin / GNOME limitations

This release does **not** implement native GNOME Wayland clipboard monitoring.
Use an Xorg/X11 login session for automatic capture. Installing `wl-clipboard`
alone cannot add the missing compositor protocol. An offline daemon needs its
user-service journal examined separately; this release does not claim to resolve
the reported remote startup failure without those logs.

## Install or update

```bash
curl -fL https://raw.githubusercontent.com/Mersad-Moghaddam/clipdeck/v0.5.2/packaging/install.sh -o /tmp/clipdeck-install.sh &&
bash /tmp/clipdeck-install.sh --version 0.5.2 --with-ui
```

Run as your normal desktop user, not root. Use `--daemon-only` for the Omarchy
plugin. GUI: Python 3.12+ and compatible Qt libraries required. Native DEB/RPM/APK
packages contain the daemon/CLI, not the standalone popup. Existing history is
kept. See `docs/desktop-support.md` for dependencies and tested scope.
