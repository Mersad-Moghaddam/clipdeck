"""Check the native Qt plugin without risking an abort in the installer."""
import os
from pathlib import Path
import subprocess
from PySide6.QtCore import QLibraryInfo


def check_native_libraries(platform):
    plugin = Path(QLibraryInfo.path(QLibraryInfo.PluginsPath)) / 'platforms' / ('libqxcb.so' if platform == 'xcb' else 'libqwayland.so')
    result = subprocess.run(['ldd', str(plugin)], capture_output=True, text=True, timeout=10)
    missing = [line.strip() for line in result.stdout.splitlines() if 'not found' in line]
    if result.returncode or missing:
        raise RuntimeError('Native Qt dependencies missing: ' + '; '.join(missing or [str(plugin)]))


if __name__ == '__main__':
    session = os.environ.get('XDG_SESSION_TYPE', '')
    platform = 'wayland' if session == 'wayland' or (not session and os.environ.get('WAYLAND_DISPLAY')) else 'xcb'
    check_native_libraries(platform)
    if os.environ.get('DISPLAY' if platform == 'xcb' else 'WAYLAND_DISPLAY'):
        env = dict(os.environ, QT_QPA_PLATFORM=platform, QT_QUICK_BACKEND='software')
        subprocess.run([os.sys.executable, '-m', 'clipdeck_ui', '--smoke-test'], env=env, check=True, timeout=15)
