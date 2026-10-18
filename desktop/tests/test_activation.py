import unittest
from clipdeck_ui.activation import active_target, i3_configuration


class ActivationTests(unittest.TestCase):
    def test_wayland_does_not_probe_xwayland(self):
        self.assertEqual(active_target(env={"XDG_SESSION_TYPE": "wayland", "DISPLAY": ":0"}), "")

    def test_i3_snippet_matches_app_identity(self):
        snippet = i3_configuration()
        self.assertIn('class="clipdeck-ui"', snippet)
        self.assertIn("floating enable", snippet)
        self.assertIn("exec --no-startup-id clipdeck-ui", snippet)
