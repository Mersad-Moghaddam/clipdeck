import unittest
from clipdeck_ui.startup import session_environment


class StartupTests(unittest.TestCase):
    def test_i3_display_is_imported_without_unrelated_secrets(self):
        env = {"DISPLAY": ":0", "XAUTHORITY": "/home/user/.Xauthority", "XDG_SESSION_TYPE": "x11", "API_TOKEN": "secret"}
        self.assertEqual(session_environment(env), {"DISPLAY": ":0", "XAUTHORITY": "/home/user/.Xauthority", "XDG_SESSION_TYPE": "x11"})

    def test_wayland_environment_keeps_compositor_identity(self):
        env = {"WAYLAND_DISPLAY": "wayland-1", "HYPRLAND_INSTANCE_SIGNATURE": "abc", "XDG_SESSION_TYPE": "wayland"}
        self.assertEqual(session_environment(env), env)
