"""Run separately with QT_QPA_PLATFORM=offscreen (requires QGuiApplication)."""
import os
import subprocess
import sys
import unittest
from pathlib import Path


class WindowTests(unittest.TestCase):
    def test_core_keyboard_actions_are_available(self):
        source = (Path(__file__).parents[1] / 'clipdeck_ui/qml/Main.qml').read_text()
        for sequence in ('Shift+Return', 'Shift+Delete', 'Ctrl+T', 'Alt+', 'Ctrl+U'):
            self.assertIn(sequence, source)
        self.assertIn('clearDialog.open()', source)
        self.assertIn('deck.paste_plain', source)

    def test_inspector_exposes_source_age_and_size(self):
        source = (Path(__file__).parents[1] / 'clipdeck_ui/qml/Main.qml').read_text()
        for field in ('deck.selected.source_app', 'deck.selected.size_bytes', 'deck.selected.created_at'):
            self.assertIn(field, source)
    def test_buttons_use_overlay_tokens_not_platform_chrome(self):
        qml = Path(__file__).parents[1] / 'clipdeck_ui/qml'
        self.assertTrue((qml / 'DeckButton.qml').exists())
        self.assertNotIn('Button {', (qml / 'Main.qml').read_text().replace('DeckButton {', ''))
    def test_qml_popup_loads_without_omarchy(self):
        env = dict(os.environ, QT_QPA_PLATFORM="offscreen", QT_QUICK_BACKEND="software")
        result = subprocess.run([sys.executable, "-m", "clipdeck_ui", "--smoke-test"], env=env, capture_output=True, text=True, timeout=15)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("ClipDeck window loaded", result.stdout)
        self.assertNotIn('Delegate must be of Item type', result.stderr)
        self.assertNotIn('Binding loop detected', result.stderr)
