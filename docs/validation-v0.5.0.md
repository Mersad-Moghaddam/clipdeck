# v0.5.0 validation log

Date: 2026-10-18. These results describe executed checks, not universal support.

## Host and nested desktop

- Go `test -race ./...` and `vet ./...`: passed.
- Python frontend suite: 32 tests passed after font/theme provisioning.
- Review fix pass: 35 tests passed after native-plugin preflight and service
  restart regressions; target-bound paste verified in nested i3. Installer late
  restart failure and TERM interruption both restore prior binaries/unit.
- Xvfb: real UTF-8 and PNG clipboard ownership/readback passed.
- i3 4.25.1 on a private Xvfb server: floating popup identity, second-launch
  activation and native backend focus-restored UTF-8 paste into Qt editor passed.
- Installer: dry-run/network isolation, archive rejection, binary rollback,
  GUI-mode/uninstall, environment import, cross-home service isolation, and
  failed-GUI environment cleanup passed.
- Frozen source GUI install/launch/uninstall with a prefix containing spaces
  passed; no installed host binaries replaced.

## Ubuntu 24.04 amd64 userspace

Ubuntu Base 24.04.3 root filesystem runs in an unprivileged bubblewrap namespace,
separate from the host's package manager. Dependencies were installed from Ubuntu
repositories. Namespace uid/group limitations required package-setup adjustments
inside the test root only. This is userspace validation, not a full logged-in
Ubuntu/GNOME/Zorin desktop session.

- Generated v0.5.0 `.deb` installed; `clipdeck version` returned v0.5.0.
- Its native dependency `wl-clipboard` installed and package configuration
  completed; systemd graphical-session symlink and package file list checked.
- Wheel installation and installed `clipdeck-ui --smoke-test` passed on
  Python 3.12/PySide6 6.12. All 32 frontend tests passed in the Ubuntu userspace.
- Minimal-root testing identified GLib/D-Bus/XKB runtime dependencies; dependency
  instructions now include them. D-Bus service-package ownership setup cannot
  complete in this single-uid namespace; runtime libraries loaded successfully.
- Native XCB loading additionally identified missing `libxcb-icccm4` and
  `libxcb-keysyms1`; these are now explicit dependencies in the setup guide.
- After installing those libraries, the installed wheel loaded a native XCB
  window on a private Xvfb server successfully.

## Artifact inspection

Local amd64 deb/rpm/apk builds succeeded. RPM/APK contain the daemon binaries;
the standalone wheel contains QML, regular/bold fonts and OFL license.
RPM and Alpine live installation have not been executed. ARM64 execution,
Zorin desktop login, and alternate Wayland compositor behavior remain unverified.
Published artifact checksums are not verified until actual publication/download.

## Visual comparison

Rendered screenshot `/tmp/opencode/clipdeck-standalone-final.png` was compared
with the existing plugin's `screenshots/dark.png`. Font/palette match work is
present; generic platform buttons were identified and replaced with shared
theme-token controls. A second render `/tmp/opencode/clipdeck-parity.png` shows
the shared typeface, palette, two-column proportions, inspector metadata and
quiet action controls. Standalone adds native-window semantics rather than a
full-screen shell scrim. Plain paste, clear confirmation, category cycling,
quick slots and transform shortcuts are wired; QML warnings found during the
render were fixed and added to the smoke-test assertions. The frontend suite
now contains 40 passing tests on the host and Ubuntu userspace.
