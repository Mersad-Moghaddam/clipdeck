"""Desktop popup entry point; no Omarchy or Quickshell runtime imports."""
import argparse
import os
from pathlib import Path
import sys

from PySide6.QtCore import QTimer, QUrl
from PySide6.QtGui import QGuiApplication, QFontDatabase, QFont
from PySide6.QtQml import QQmlApplicationEngine

from .ipc import DaemonClient
from .model import PopupController
from .theme import resolve_theme
from .activation import active_target
from .instance import InstanceServer


def main():
    parser = argparse.ArgumentParser(description="ClipDeck clipboard popup")
    parser.add_argument("--smoke-test", action="store_true", help=argparse.SUPPRESS)
    parser.add_argument("--socket", help="Override the daemon's Unix socket")
    args = parser.parse_args()
    target = "" if args.smoke_test else active_target()
    app = QGuiApplication(sys.argv[:1])
    app.setApplicationName("clipdeck-ui")
    app.setDesktopFileName("clipdeck")
    for font in (Path(__file__).parent / "fonts").glob("*.ttf"):
        QFontDatabase.addApplicationFont(str(font))
    tokens = resolve_theme()
    app_font = QFont(tokens["fontFamily"])
    app_font.setPixelSize(tokens["body"])
    app.setFont(app_font)
    runtime = os.environ.get("XDG_RUNTIME_DIR", f"/run/user/{os.getuid()}")
    instance = None
    if not args.smoke_test:
        instance = InstanceServer(f"{runtime}/clipdeck-ui.sock")
        if instance.activate(target):
            return 0
        if not instance.listen():
            print("Cannot establish the ClipDeck activation socket; another launch may be in progress.", file=sys.stderr)
            return 1
        app.setQuitOnLastWindowClosed(False)
    client = DaemonClient(args.socket or f"{runtime}/clipdeck/clipdeck.sock")
    controller = PopupController(client)
    controller.set_target(target)
    if instance:
        def reopen(window_id):
            controller.set_target(window_id)
            controller.open()
            controller.search("", "All")
            client.connect_to_daemon()
            if engine.rootObjects():
                engine.rootObjects()[0].raise_()
                engine.rootObjects()[0].requestActivate()
        instance.activated.connect(reopen)
    engine = QQmlApplicationEngine()
    engine.rootContext().setContextProperty("deck", controller)
    engine.rootContext().setContextProperty("theme", tokens)
    engine.load(QUrl.fromLocalFile(str(Path(__file__).parent / "qml" / "Main.qml")))
    if not engine.rootObjects():
        return 1
    controller.open()
    if args.smoke_test:
        print("ClipDeck window loaded", flush=True)
        QTimer.singleShot(150, app.quit)
    else:
        client.connect_to_daemon()
    return app.exec()


if __name__ == "__main__":
    sys.exit(main())
