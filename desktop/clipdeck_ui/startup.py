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
    if values:
        subprocess.run(["systemctl", "--user", "import-environment", *values], check=True, timeout=5)
    subprocess.run(["systemctl", "--user", "enable", "--now", "clipdeckd.service"], check=True, timeout=10)
