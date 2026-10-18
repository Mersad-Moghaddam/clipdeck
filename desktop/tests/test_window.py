"""Run separately with QT_QPA_PLATFORM=offscreen (requires QGuiApplication)."""
import os
import subprocess
import sys
import unittest


class WindowTests(unittest.TestCase):
    def test_qml_popup_loads_without_omarchy(self):
        env = dict(os.environ, QT_QPA_PLATFORM="offscreen", QT_QUICK_BACKEND="software")
        result = subprocess.run([sys.executable, "-m", "clipdeck_ui", "--smoke-test"], env=env, capture_output=True, text=True, timeout=15)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("ClipDeck window loaded", result.stdout)
