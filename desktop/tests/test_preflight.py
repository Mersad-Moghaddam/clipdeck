import unittest
from unittest.mock import patch
from types import SimpleNamespace
from clipdeck_ui.preflight import check_native_libraries


class PreflightTests(unittest.TestCase):
    def test_missing_xcb_dependency_rejects_install(self):
        with patch('clipdeck_ui.preflight.subprocess.run', return_value=SimpleNamespace(returncode=0, stdout='libxcb-keysyms.so.1 => not found\n')):
            with self.assertRaisesRegex(RuntimeError, 'libxcb-keysyms'):
                check_native_libraries('xcb')

    def test_resolved_dependencies_pass(self):
        with patch('clipdeck_ui.preflight.subprocess.run', return_value=SimpleNamespace(returncode=0, stdout='libc.so => /lib/libc.so\n')):
            check_native_libraries('xcb')
