import unittest
from PySide6.QtCore import QObject, Signal
from clipdeck_ui.model import PopupController


class FakeClient(QObject):
    response = Signal(str, object)
    failed = Signal(str, str)
    event_received = Signal(str, object)
    connected = Signal()

    def __init__(self):
        super().__init__()
        self.calls = []
        self.connections = 0

    def connect_to_daemon(self):
        self.connections += 1

    def call(self, method, params):
        request_id = str(len(self.calls) + 1)
        self.calls.append((request_id, method, params))
        return request_id


class PopupTests(unittest.TestCase):
    def setUp(self):
        self.client = FakeClient()
        self.popup = PopupController(self.client)

    def test_stale_query_cannot_replace_latest_search(self):
        self.popup.search("old", "All")
        self.popup.search("new", "All")
        self.client.response.emit("2", {"items": [{"id": "new"}], "has_more": False})
        self.client.response.emit("1", {"items": [{"id": "old"}], "has_more": False})
        self.assertEqual(self.popup.items, [{"id": "new"}])

    def test_copy_hides_only_on_confirmed_success(self):
        self.popup.open()
        self.popup.copy_item("a")
        self.assertTrue(self.popup.visible)
        request_id = self.client.calls[-1][0]
        self.client.failed.emit(request_id, "Copy failed")
        self.assertTrue(self.popup.visible)
        self.assertEqual(self.popup.message, "Copy failed")
        self.popup.copy_item("a")
        self.client.response.emit(self.client.calls[-1][0], {"success": True})
        self.assertFalse(self.popup.visible)

    def test_sensitive_item_never_requests_unmasked_preview(self):
        self.popup.select_item({"id": "secret", "sensitive": True, "masked_content": "••••", "content": "unsafe"})
        self.assertEqual(self.client.calls, [])
        self.assertEqual(self.popup.preview, "••••")

    def test_paste_request_is_not_confirmation(self):
        self.popup.open()
        self.popup.paste_item("a")
        self.client.response.emit(self.client.calls[-1][0], {"copied": True, "paste_requested": True})
        self.assertTrue(self.popup.visible)

    def test_paste_includes_original_target(self):
        self.popup.set_target("42")
        self.popup.paste_item("a")
        self.assertEqual(self.client.calls[-1][2]["target_window"], "42")

    def test_image_preview_is_available_without_interpreting_text_as_markup(self):
        self.popup.select_item({"id": "image", "type": "image", "thumbnail_path": "/home/user/thumb.png"})
        self.assertEqual(self.popup.imageSource, "file:///home/user/thumb.png")
        self.popup.select_item({"id": "secret", "type": "image", "sensitive": True, "thumbnail_path": "/home/user/private.png"})
        self.assertEqual(self.popup.imageSource, "")

    def test_transform_copy_uses_daemon_transform(self):
        self.popup.copy_transformed("a", "uppercase")
        self.assertEqual(self.client.calls[-1][1:], ("copy", {"id": "a", "transform": "uppercase"}))

    def test_connection_failure_can_be_retried(self):
        self.popup.retry_connection()
        self.assertEqual(self.client.connections, 1)
