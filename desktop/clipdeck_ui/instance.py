"""User-local activation socket for re-opening a hidden popup."""
from PySide6.QtCore import QObject, Signal
from PySide6.QtNetwork import QLocalServer, QLocalSocket


class InstanceServer(QObject):
    activated = Signal(str)

    def __init__(self, path, parent=None):
        super().__init__(parent)
        self.path = path
        self.server = QLocalServer(self)
        self.server.setSocketOptions(QLocalServer.UserAccessOption)
        self.server.newConnection.connect(self._accept)
        self.connections = set()

    def activate(self, target):
        socket = QLocalSocket()
        socket.connectToServer(self.path)
        if not socket.waitForConnected(250):
            return False
        socket.write((target + "\n").encode("ascii"))
        socket.waitForBytesWritten(250)
        socket.disconnectFromServer()
        return True

    def listen(self):
        return self.server.listen(self.path)

    def _accept(self):
        while self.server.hasPendingConnections():
            socket = self.server.nextPendingConnection()
            self.connections.add(socket)
            socket.readyRead.connect(lambda socket=socket: self._read(socket))
            socket.disconnected.connect(lambda socket=socket: self._drop(socket))
            self._read(socket)

    def _read(self, socket):
        if socket.bytesAvailable() > 64:
            socket.abort()
        elif socket.canReadLine():
            value = bytes(socket.readLine(64)).decode("ascii", errors="ignore").strip()
            if value == "" or (value.isdecimal() and len(value) <= 20):
                self.activated.emit(value)
            socket.disconnectFromServer()

    def _drop(self, socket):
        self.connections.discard(socket)
        socket.deleteLater()

    def close(self):
        self.server.close()
