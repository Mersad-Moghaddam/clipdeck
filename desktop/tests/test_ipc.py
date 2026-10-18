import json
import unittest

from PySide6.QtCore import QCoreApplication
from clipdeck_ui.ipc import DaemonClient

APP = QCoreApplication.instance() or QCoreApplication([])


class ClientTests(unittest.TestCase):
    def setUp(self):
        self.client = DaemonClient("/nonexistent/clipdeck-test.sock")
        self.responses = []
        self.errors = []
        self.events = []
        self.client.response.connect(lambda *args: self.responses.append(args))
        self.client.failed.connect(lambda *args: self.errors.append(args))
        self.client.event_received.connect(lambda *args: self.events.append(args))

    def test_partial_frames_and_events(self):
        self.client.feed(b'{"id":"a","result":{"items":')
        self.assertEqual(self.responses, [])
        self.client.feed(b'[]}}\n{"event":"item_added","data":{"id":"b"}}\n')
        self.assertEqual(self.responses, [("a", {"items": []})])
        self.assertEqual(self.events, [("item_added", {"id": "b"})])

    def test_rpc_error_is_recoverable(self):
        self.client.feed(b'{"id":"a","error":{"message":"Unavailable"}}\n')
        self.client.feed(b'{"id":"b","result":{"success":true}}\n')
        self.assertEqual(self.errors, [("a", "Unavailable")])
        self.assertEqual(self.responses, [("b", {"success": True})])

    def test_malformed_frame_does_not_prevent_next_response(self):
        self.client.feed(b'not-json\n{"id":"b","result":{}}\n')
        self.assertEqual(len(self.errors), 1)
        self.assertEqual(self.responses, [("b", {})])

    def test_oversized_unterminated_frame_is_discarded(self):
        self.client.feed(b"x" * (8 * 1024 * 1024 + 1))
        self.assertEqual(len(self.errors), 1)
        self.assertEqual(self.client.buffer_size, 0)
