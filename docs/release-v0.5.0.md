# v0.5.0 release notes

## Standalone desktop popup

- Qt Quick application independent of Omarchy and Quickshell.
- Searchable history, categories, pinned items, plain-text and image previews,
  copy/plain paste, deletion, text transforms, merge-copy and confirmed history clearing.
- Single-instance reopening and actionable connection/startup feedback.
- Bundled JetBrainsMono Nerd Font with SIL OFL license; live Omarchy palette
  and typography overrides when present.

## Native X11 backend

- xclip capture and clipboard ownership for text, PNG and file selections.
- xdotool target focus restoration with xprop popup-target rejection.
- Session selection honors explicit XDG_SESSION_TYPE, including XWayland hosts.
- Existing Omarchy overlay and Wayland plugin behavior are not redesigned.
- Standalone Wayland automatic paste is deliberately copy-only where safe
  target restoration cannot be confirmed.

## Installation

- Explicit `--with-ui` / `--daemon-only` modes, private Python environment,
  checksummed wheel/helper, desktop launcher and session startup helper.
- Optional i3 configuration snippets; no automatic edits to i3 configuration.
- GUI staging failures preserve prior daemon binaries and clean partial venvs.
- Uninstall preserves history/configuration and avoids stopping unrelated user
  services when no installer-owned unit exists in the selected home.

## Validation and limitations

Local validation includes Go race/vet, frontend/offscreen tests, real Xvfb text
and PNG clipboard roundtrips, nested i3 popup/activation/UTF-8 paste delivery,
and installer regression tests. Ubuntu 24.04 userspace wheel installation,
native XCB launch, frontend suite and daemon deb installation were executed.
Independent review findings were addressed with regression tests.

GNOME Wayland does not expose the clipboard-watch protocol required here;
choose an X11 session. No universal compositor/distribution compatibility is
claimed. Native daemon packages and the optional GUI wheel remain separate
deliverables. AUR listing and marketplace promotion are external processes.
