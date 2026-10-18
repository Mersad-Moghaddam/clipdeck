"""Reopening must preserve the filters still displayed by the window."""
import unittest
from test_model import FakeClient
from clipdeck_ui.model import PopupController


class ReopenTests(unittest.TestCase):
    def test_reopen_preserves_search_and_category_and_updates_paste_target(self):
        client = FakeClient()
        popup = PopupController(client)
        popup.search("needle", "Pinned")
        popup.dismiss()
        popup.reopen("99")
        self.assertTrue(popup.visible)
        self.assertEqual(client.calls[-1][1], "query")
        self.assertEqual(client.calls[-1][2]["query"], "needle")
        self.assertTrue(client.calls[-1][2]["pinned_only"])
        popup.paste_item("clip")
        self.assertEqual(client.calls[-1][2]["target_window"], "99")
