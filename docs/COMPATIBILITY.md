# Desktop compatibility — verification in progress

Packages and static binaries do not guarantee desktop protocol support.

| Environment | Capture | Rich frontend | v0.4 live validation |
|---|---|---|---|
| Omarchy / Hyprland | existing wl-paste watcher | Omarchy plugin | pending |
| Sway | requires compatible clipboard protocols | external picker | not tested |
| GNOME Wayland | not established by wl-clipboard installation | external picker only | not tested |
| KDE Wayland | protocol/version dependent | external picker only | not tested |
| X11 | no native capture backend in this release scope | menu availability does not implement capture | unsupported capture |
| Alpine/non-systemd | protocol dependent; manual daemon startup | external picker | not tested |

The CLI picker requires the running query-capable daemon and an installed menu.
Copy-only is the default. Paste injection is optional and successful command
execution does not confirm delivery to the target application. Source attribution
is currently Hyprland-specific; do not assume app exclusions work elsewhere.
