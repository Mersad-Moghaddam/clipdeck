"""Nonblocking newline-delimited JSON client for the private daemon socket."""

import json

from PySide6.QtCore import QObject, QTimer, Signal
from PySide6.QtNetwork import QLocalSocket


class DaemonClient(QObject):
    response = Signal(str, object)
    failed = Signal(str, str)
    event_received = Signal(str, object)
    connected = Signal()
    MAX_FRAME = 8 * 1024 * 1024

    def __init__(self, socket_path, parent=None):
        super().__init__(parent)
        self.socket_path = socket_path
        self.socket = QLocalSocket(self)
        self.socket.readyRead.connect(self._read)
        self.socket.connected.connect(self.connected)
        self.socket.errorOccurred.connect(self._socket_error)
        self._buffer = bytearray()
        self._sequence = 0
        self._pending = {}
        self._clock = QTimer(self)
        self._clock.setInterval(1000)
        self._clock.timeout.connect(self._expire)
        self._clock.start()

    @property
    def buffer_size(self):
        return len(self._buffer)

    def connect_to_daemon(self):
        if self.socket.state() == QLocalSocket.UnconnectedState:
            self._buffer.clear()
            self.socket.connectToServer(self.socket_path)

    def call(self, method, params=None):
        import time
        self._sequence += 1
        request_id = f"ui-{self._sequence}"
        if self.socket.state() != QLocalSocket.ConnectedState:
            QTimer.singleShot(0, lambda: self.failed.emit(request_id, "Daemon is offline. Start ClipDeck and retry."))
            return request_id
        frame = json.dumps({"id": request_id, "method": method, "params": params or {}}, ensure_ascii=False).encode() + b"\n"
        self._pending[request_id] = time.monotonic() + 5
        self.socket.write(frame)
        return request_id

    def _read(self):
        self.feed(bytes(self.socket.readAll()))

    def feed(self, data):
        self._buffer.extend(data)
        while b"\n" in self._buffer:
            line, _, rest = self._buffer.partition(b"\n")
            self._buffer = bytearray(rest)
            try:
                if len(line) > self.MAX_FRAME:
                    raise ValueError("Daemon response exceeds size limit")
                frame = json.loads(line)
                if not isinstance(frame, dict):
                    raise ValueError("Invalid daemon response")
                if isinstance(frame.get("event"), str):
                    self.event_received.emit(frame["event"], frame.get("data", {}))
                    continue
                request_id = frame.get("id")
                if not isinstance(request_id, str):
                    raise ValueError("Daemon response has no request ID")
                self._pending.pop(request_id, None)
                if frame.get("error"):
                    error = frame["error"]
                    message = error.get("message", "Daemon request failed") if isinstance(error, dict) else "Invalid daemon error"
                    self.failed.emit(request_id, str(message))
                else:
                    self.response.emit(request_id, frame.get("result", {}))
            except (ValueError, TypeError, UnicodeError) as error:
                self.failed.emit("", str(error))
        if len(self._buffer) > self.MAX_FRAME:
            self._buffer.clear()
            self.failed.emit("", "Daemon response exceeds size limit")
            self.socket.abort()

    def _socket_error(self, _error):
        pending, self._pending = self._pending, {}
        for request_id in pending:
            self.failed.emit(request_id, "Daemon connection lost. Retry the action.")
        self.failed.emit("", self.socket.errorString())

    def _expire(self):
        import time
        for request_id, deadline in list(self._pending.items()):
            if deadline <= time.monotonic():
                del self._pending[request_id]
                self.failed.emit(request_id, "Daemon request timed out. Retry the action.")
