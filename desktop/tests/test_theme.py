import unittest
import tempfile
from pathlib import Path

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

    def test_live_palette_and_user_override(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            theme = root / "state/omarchy/current/theme"
            theme.mkdir(parents=True)
            (theme / "colors.toml").write_text('accent = "#123456"\n')
            (theme / "shell.toml").write_text('[menu]\nbackground = "#112233"\ntext = "#abcdef"\n[font]\nbase-size = 24\n')
            config = root / "config/omarchy"
            config.mkdir(parents=True)
            (config / "shell.toml").write_text('[font]\nbody = 17\n')
            result = resolve_theme({"XDG_STATE_HOME": str(root / "state"), "XDG_CONFIG_HOME": str(root / "config")})
            self.assertEqual(result["background"], "#112233")
            self.assertEqual(result["foreground"], "#abcdef")
            self.assertEqual(result["accent"], "#123456")
            self.assertEqual(result["body"], 17)
            self.assertEqual(result["title"], 28)

    def test_bundled_font_and_license_exist(self):
        assets = Path(__file__).parents[1] / "clipdeck_ui/fonts"
        self.assertTrue((assets / "JetBrainsMonoNerdFont-Regular.ttf").is_file())
        self.assertIn("SIL OPEN FONT LICENSE", (assets / "OFL.txt").read_text())
