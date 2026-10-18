"""Render a deterministic standalone reference; run with QT_QPA_PLATFORM=offscreen."""
from pathlib import Path
import sys
from PySide6.QtCore import QTimer, QUrl
from PySide6.QtGui import QFont, QFontDatabase, QGuiApplication
from PySide6.QtQml import QQmlApplicationEngine
from PySide6.QtQuick import QQuickWindow
from clipdeck_ui.ipc import DaemonClient
from clipdeck_ui.model import PopupController
from clipdeck_ui.theme import resolve_theme

app = QGuiApplication([])
package = Path(__file__).resolve().parents[1] / 'desktop/clipdeck_ui'
for font in (package / 'fonts').glob('*.ttf'):
    QFontDatabase.addApplicationFont(str(font))
tokens = resolve_theme({})
font = QFont(tokens['fontFamily'])
font.setPixelSize(tokens['body'])
app.setFont(font)
client = DaemonClient('/nonexistent')
deck = PopupController(client)
deck._items = [
    {'id': 'code', 'type': 'code', 'source_app': 'editor', 'preview': 'package main', 'content': 'package main\n\nfunc main() {\n    println("ClipDeck")\n}', 'pinned': True, 'size_bytes': 61, 'created_at': '2026-10-18T00:00:00Z'},
    {'id': 'link', 'type': 'link', 'source_app': 'firefox', 'preview': 'omarchy.org  omarchy.org/docs', 'size_bytes': 28},
    {'id': 'text', 'type': 'text', 'source_app': 'terminal', 'preview': 'Clipboard history on X11', 'size_bytes': 23},
]
deck._selected = deck._items[0]
deck._preview = deck._selected['content']
deck.open()
engine = QQmlApplicationEngine()
engine.rootContext().setContextProperty('deck', deck)
engine.rootContext().setContextProperty('theme', tokens)
engine.load(QUrl.fromLocalFile(str(package / 'qml/Main.qml')))

def save():
    image = engine.rootObjects()[0].grabWindow()
    success = image.save(sys.argv[1])
    engine.clearComponentCache()
    app.exit(0 if success else 1)

QTimer.singleShot(300, save)
sys.exit(app.exec())
