# ClipDeck v0.5.1

Fixes the standalone popup's reopening behavior: the active search and category
are preserved to match the visible controls, while the paste target is updated
for the newly active application.

Verification: 41 frontend tests, Go race/vet and nested i3 target-bound UTF-8
paste. This patch does not change the existing Omarchy overlay.

Install using the public version-pinned installer with `--version 0.5.1` and
`--with-ui` for the standalone popup, or `--daemon-only` for the Omarchy plugin.
The optional GUI requires Python 3.12+. See README for desktop dependencies,
support boundaries, upgrade and recovery instructions.
