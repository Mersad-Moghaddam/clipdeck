"""Typography mirrors Omarchy Commons/Style.qml; no shell runtime required."""

import os


def resolve_theme(env=None):
    env = os.environ if env is None else env
    family = env.get("OMARCHY_MENU_FONT", "").strip()
    if not family or any(ord(char) < 32 for char in family) or len(family) > 200:
        family = "JetBrainsMono Nerd Font"
    return {
        "fontFamily": family,
        "codeFamily": "JetBrainsMono Nerd Font",
        "caption": 10, "bodySmall": 11, "body": 12, "subtitle": 13, "title": 14,
        "background": "#100f0d", "surface": "#191713", "foreground": "#b9af93",
        "muted": "#847b64", "accent": "#be402c", "selected": "#242019",
        "border": "#2c2820", "radius": 8,
    }
