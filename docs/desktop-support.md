# Desktop support (v0.5.0 development)

The standalone popup is under development. These are implementation and test
results, not a claim that every distribution or desktop has been certified.

| Environment | Capture/copy | Automatic paste | Evidence |
|---|---|---|---|
| Omarchy / Hyprland | Existing wl-clipboard backend | Existing plugin integration | Go regression suite; existing overlay unchanged |
| X11 | xclip, text and PNG | xdotool, original target required | Real clipboard roundtrip under Xvfb; native WM acceptance pending |
| i3 / X11 | Same X11 backend | Focus restore before Ctrl+V | Floating/shortcut configuration provided; real i3 session test pending |
| Ubuntu / Zorin | Depends on session selected at login | X11 target restore; Wayland copy-only fallback | Clean distro install validation pending |
| Other Wayland desktops | wl-clipboard; capture protocol must be available | Standalone app offers copy-only fallback | Per-compositor testing required; GNOME does not provide wl-paste watch protocol |

## i3 configuration

After installing the standalone app, add these **optional** lines to your i3
configuration (`~/.config/i3/config`) and reload i3. The installer does not edit
your configuration or replace an existing shortcut.

```i3
for_window [class="clipdeck-ui"] floating enable, move position center
bindsym $mod+v exec --no-startup-id clipdeck-ui
```

If `$mod+v` is already assigned, choose a different binding. Run `clipdeck-ui`
again to reopen the same hidden popup. After successful copy/paste it hides;
clipboard or focus failures leave it open with a message. `xdotool` must be able
to activate the original window before ClipDeck will inject Ctrl+V.

Install dependencies on Debian/Ubuntu/Zorin:

```bash
sudo apt install python3 python3-venv xclip xdotool x11-utils
```

On Arch/i3:

```bash
sudo pacman -S python xclip xdotool xorg-xprop
```

## Typography

The standalone app mirrors Omarchy's 12px typography scale: caption 10,
body-small 11, body 12, subtitle 13, and title 14. It targets JetBrainsMono Nerd
Font and respects `OMARCHY_MENU_FONT`; missing-font provisioning and visual
acceptance are release gates, not yet complete. No font assets are redistributed
by the current development build.

## Startup

The existing systemd user service is attached to `graphical-session.target`.
Some bare i3 sessions do not activate this target or import DISPLAY into the
user manager: startup for those sessions is still being implemented. Do not use
root/system-wide daemon startup as a workaround. Non-systemd startup support is
also a release gate.
