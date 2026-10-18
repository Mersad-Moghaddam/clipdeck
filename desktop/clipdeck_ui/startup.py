"""Explicit desktop-session service setup, including bare i3 sessions."""
import os
import subprocess


def session_environment(env=None):
    env = os.environ if env is None else env
    keys = ("DISPLAY", "XAUTHORITY", "WAYLAND_DISPLAY", "XDG_SESSION_TYPE", "XDG_CURRENT_DESKTOP", "HYPRLAND_INSTANCE_SIGNATURE")
    return {key: env[key] for key in keys if env.get(key)}


def enable_service():
    """User-triggered action; no root, shell, downloads, or config rewriting."""
    values = session_environment()
    for key in ("DISPLAY", "XAUTHORITY", "WAYLAND_DISPLAY", "XDG_SESSION_TYPE", "XDG_CURRENT_DESKTOP", "HYPRLAND_INSTANCE_SIGNATURE"):
        if key not in values:
            subprocess.run(["systemctl", "--user", "unset-environment", key], check=True, timeout=5)
    if values:
        subprocess.run(["systemctl", "--user", "import-environment", *values], check=True, timeout=5)
    subprocess.run(["systemctl", "--user", "enable", "clipdeckd.service"], check=True, timeout=10)
    subprocess.run(["systemctl", "--user", "restart", "clipdeckd.service"], check=True, timeout=10)
