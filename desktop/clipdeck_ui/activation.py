"""Native session activation helpers; never modify window-manager config."""
import os
import subprocess


def active_target(env=None):
    env = os.environ if env is None else env
    session = env.get("XDG_SESSION_TYPE", "")
    if session == "wayland" or (session != "x11" and env.get("WAYLAND_DISPLAY")) or not env.get("DISPLAY"):
        return ""
    try:
        result = subprocess.run(["xdotool", "getactivewindow"], capture_output=True, text=True, timeout=1, check=True)
        value = result.stdout.strip()
        return value if value.isascii() and value.isdecimal() and int(value) > 0 else ""
    except (OSError, subprocess.SubprocessError):
        return ""


def i3_configuration():
    return 'for_window [class="clipdeck-ui"] floating enable, move position center\nbindsym $mod+v exec --no-startup-id clipdeck-ui\n'
