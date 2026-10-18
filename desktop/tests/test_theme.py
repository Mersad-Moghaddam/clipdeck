import unittest

from clipdeck_ui.theme import resolve_theme


class ThemeTests(unittest.TestCase):
    def test_omarchy_typography_defaults(self):
        theme = resolve_theme(env={})
        self.assertEqual(theme["fontFamily"], "JetBrainsMono Nerd Font")
        self.assertEqual([theme[key] for key in ("caption", "bodySmall", "body", "subtitle", "title")], [10, 11, 12, 13, 14])

    def test_menu_font_override(self):
        self.assertEqual(resolve_theme(env={"OMARCHY_MENU_FONT": "My Mono"})["fontFamily"], "My Mono")

    def test_invalid_override_falls_back(self):
        self.assertEqual(resolve_theme(env={"OMARCHY_MENU_FONT": "\n"})["fontFamily"], "JetBrainsMono Nerd Font")
