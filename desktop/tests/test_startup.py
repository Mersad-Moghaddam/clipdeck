import unittest
from unittest.mock import patch
from clipdeck_ui.startup import session_environment, enable_service


class StartupTests(unittest.TestCase):
    def test_recovery_restarts_existing_daemon_after_import(self):
        with patch('clipdeck_ui.startup.subprocess.run') as run:
            enable_service()
        self.assertEqual(run.call_args_list[-1].args[0], ['systemctl', '--user', 'restart', 'clipdeckd.service'])
    def test_i3_display_is_imported_without_unrelated_secrets(self):
        env = {"DISPLAY": ":0", "XAUTHORITY": "/home/user/.Xauthority", "XDG_SESSION_TYPE": "x11", "API_TOKEN": "secret"}
        self.assertEqual(session_environment(env), {"DISPLAY": ":0", "XAUTHORITY": "/home/user/.Xauthority", "XDG_SESSION_TYPE": "x11"})

    def test_wayland_environment_keeps_compositor_identity(self):
        env = {"WAYLAND_DISPLAY": "wayland-1", "HYPRLAND_INSTANCE_SIGNATURE": "abc", "XDG_SESSION_TYPE": "wayland"}
        self.assertEqual(session_environment(env), env)
