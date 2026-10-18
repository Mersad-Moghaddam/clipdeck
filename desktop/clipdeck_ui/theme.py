"""Typography mirrors Omarchy Commons/Style.qml; no shell runtime required."""

import os
import re
from pathlib import Path
try:
    import tomllib
except ImportError:
    import tomli as tomllib


def _config(path):
    try:
        if path.stat().st_size > 262144:
            return {}
        with path.open("rb") as source:
            return tomllib.load(source)
    except (OSError, ValueError):
        return {}


def resolve_theme(env=None):
    env = os.environ if env is None else env
    family = env.get("OMARCHY_MENU_FONT", "").strip()
    if not family or any(ord(char) < 32 for char in family) or len(family) > 200:
        family = "JetBrainsMono Nerd Font"
    result = {
        "fontFamily": family,
        "codeFamily": "JetBrainsMono Nerd Font",
        "caption": 10, "bodySmall": 11, "body": 12, "subtitle": 13, "title": 14,
        "background": "#100f0d", "surface": "#191713", "foreground": "#b9af93",
        "muted": "#847b64", "accent": "#be402c", "selected": "#242019",
        "border": "#2c2820", "radius": 8,
    }
    home = Path(env.get("HOME", str(Path.home())))
    state = Path(env.get("XDG_STATE_HOME", str(home / ".local/state")))
    config = Path(env.get("XDG_CONFIG_HOME", str(home / ".config")))
    theme = state / "omarchy/current/theme"
    colors = _config(theme / "colors.toml")
    shell = _config(theme / "shell.toml")
    for section, values in _config(config / "omarchy/shell.toml").items():
        if isinstance(values, dict):
            existing = shell.get(section, {})
            shell[section] = {**(existing if isinstance(existing, dict) else {}), **values}
    menu = shell.get("menu", {})
    font = shell.get("font", {})
    menu = menu if isinstance(menu, dict) else {}
    font = font if isinstance(font, dict) else {}
    for key, value in {"background": menu.get("background"), "foreground": menu.get("text"), "accent": colors.get("accent"), "border": menu.get("border")}.items():
        if isinstance(value, str) and re.fullmatch(r"#[0-9a-fA-F]{6}", value):
            result[key] = value
    base = font.get("base-size", 12)
    base = base if type(base) in (int, float) and 6 <= base <= 48 else 12
    for key, token, size in (("caption", "caption", 10), ("bodySmall", "body-small", 11), ("body", "body", 12), ("subtitle", "subtitle", 13), ("title", "title", 14)):
        value = font.get(token, int(size * base / 12 + .5))
        if type(value) in (int, float) and 6 <= value <= 64:
            result[key] = int(value)
    return result
