# Desktop support (v0.5.0 development)

The standalone popup requires Python 3.12+ (Ubuntu 24.04 baseline). Python 3.10
ran all assertions but aborted in Qt teardown on hosted CI, so it is not a
supported GUI baseline. Older systems can use the daemon/CLI and existing plugin.

These are implementation and test results, not a claim that every distribution
or desktop has been certified.

| Environment | Capture/copy | Automatic paste | Evidence |
|---|---|---|---|
| Omarchy / Hyprland | Existing wl-clipboard backend | Existing plugin integration | Go regression suite; existing overlay unchanged |
| X11 | xclip, text and PNG | xdotool, original target required | Real clipboard roundtrip under Xvfb; native WM acceptance pending |
| i3 / X11 | Same X11 backend | Focus restore before Ctrl+V | Nested real i3/Xvfb floating popup, single-instance reactivation, and native backend UTF-8 paste delivery passed; clean distro acceptance pending |
| Ubuntu / Zorin | Depends on session selected at login | X11 target restore; Wayland copy-only fallback | Clean distro install validation pending |
| Other Wayland desktops | wl-clipboard; capture protocol must be available | Standalone app offers copy-only fallback | Per-compositor testing required; GNOME does not provide wl-paste watch protocol |

## i3 configuration

After installing the standalone app, add these **optional** lines to your i3
configuration (`~/.config/i3/config`) and reload i3. The installer does not edit
your configuration or replace an existing shortcut.

```i3
for_window [class="clipdeck-ui"] floating enable, move position center
bindsym $mod+v exec --no-startup-id clipdeck-ui
exec --no-startup-id clipdeck-session
```

If `$mod+v` is already assigned, choose a different binding. Run `clipdeck-ui`
again to reopen the same hidden popup. After successful copy/paste it hides;
clipboard or focus failures leave it open with a message. `xdotool` must be able
to activate the original window, and `xprop` must verify that the target is not
the popup, before ClipDeck will inject Ctrl+V.

Install dependencies on Debian/Ubuntu/Zorin:

```bash
sudo apt install python3 python3-venv xclip xdotool x11-utils libegl1 libopengl0 libxcb-cursor0 libglib2.0-0 libdbus-1-3 libxkbcommon-x11-0 libxcb-icccm4 libxcb-keysyms1
```

On Arch/i3:

```bash
sudo pacman -S python xclip xdotool xorg-xprop
```

## Typography

The standalone app mirrors Omarchy's 12px typography scale: caption 10,
body-small 11, body 12, subtitle 13, and title 14. It bundles regular/bold
JetBrainsMono Nerd Font under SIL OFL 1.1 and respects `OMARCHY_MENU_FONT`.
Active Omarchy palette and font-size overrides are read at launch. See
[design provenance](standalone-design.md) for configuration and licensing.

## Startup

The existing systemd user service is attached to `graphical-session.target`.
The GUI installer also installs a desktop-session autostart entry: its
`clipdeck-session` helper imports only graphical-session variables before
starting the installed user service. Bare i3 sessions that do not process XDG
autostart entries can use the `exec` line above. Without a user systemd manager,
the helper uses a private-runtime `flock` to start one session daemon. Do not use
root/system-wide daemon startup as a workaround. Clean distribution startup
verification remains a release gate.
