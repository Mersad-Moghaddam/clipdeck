"""Popup state independent of its window; all daemon actions are asynchronous."""

from PySide6.QtCore import QObject, Property, Signal, Slot, QThread
from pathlib import Path


class PopupController(QObject):
    changed = Signal()

    def __init__(self, client, parent=None):
        super().__init__(parent)
        self.client = client
        self._items = []
        self._visible = False
        self._message = ""
        self._preview = ""
        self._selected = {}
        self._requests = {}
        self._query_id = None
        self._query = ""
        self._category = "All"
        self._has_more = False
        self._target_window = ""
        client.response.connect(self._response)
        client.failed.connect(self._failed)
        client.event_received.connect(self._event)
        client.connected.connect(lambda: self.search(self._query, self._category))

    items = Property("QVariantList", lambda self: self._items, notify=changed)
    visible = Property(bool, lambda self: self._visible, notify=changed)
    message = Property(str, lambda self: self._message, notify=changed)
    preview = Property(str, lambda self: self._preview, notify=changed)
    selected = Property("QVariantMap", lambda self: self._selected, notify=changed)
    hasMore = Property(bool, lambda self: self._has_more, notify=changed)
    imageSource = Property(str, lambda self: self._image_source(), notify=changed)

    def _image_source(self):
        item = self._selected
        path = item.get("thumbnail_path") or item.get("blob_path")
        if item.get("type") != "image" or item.get("sensitive") or not isinstance(path, str) or not Path(path).is_absolute():
            return ""
        return Path(path).as_uri()

    @Slot()
    def open(self):
        self._visible = True
        self._message = ""
        self.changed.emit()

    def reopen(self, window_id):
        self.set_target(window_id)
        self.open()
        self.search(self._query, self._category)
        self.client.connect_to_daemon()

    @Slot()
    def dismiss(self):
        self._visible = False
        self.changed.emit()

    @Slot()
    def retry_connection(self):
        self._message = "Connecting to ClipDeck…"
        self.client.connect_to_daemon()
        self.changed.emit()

    @Slot()
    def enable_startup(self):
        if getattr(self, "_setup_worker", None) and self._setup_worker.isRunning():
            return
        self._message = "Enabling the installed user service…"
        self._setup_worker = ServiceSetup(self)
        self._setup_worker.result.connect(self.setup_finished)
        self._setup_worker.start()
        self.changed.emit()

    @Slot(bool, str)
    def setup_finished(self, success, message):
        self._message = message
        if success:
            self.client.connect_to_daemon()
        self.changed.emit()

    @Slot(str, str)
    def search(self, query, category):
        self._query, self._category = query, category
        types = {"Colors": ["color"], "Code": ["code", "json"], "Links": ["link"], "Images": ["image"], "Text": ["text"]}.get(category, [])
        params = {"query": query, "types": types, "pinned_only": category == "Pinned", "limit": 60, "offset": 0}
        request_id = self.client.call("query", params)
        self._query_id = request_id
        self._requests[request_id] = ("query", params)

    @Slot()
    def load_more(self):
        if not self._has_more:
            return
        if any(action == "page" and value == (self._query_id, len(self._items)) for action, value in self._requests.values()):
            return
        params = next((dict(value[1]) for key, value in self._requests.items() if key == self._query_id), None)
        if params is None:
            params = dict(self._last_params)
        params["offset"] = len(self._items)
        request_id = self.client.call("query", params)
        self._requests[request_id] = ("page", (self._query_id, params["offset"]))

    @Slot("QVariantMap")
    def select_item(self, item):
        self._selected = dict(item)
        if item.get("sensitive"):
            self._preview = item.get("masked_content") or "Sensitive content hidden"
        else:
            self._preview = item.get("content") or item.get("preview", "")
            if item.get("id"):
                request_id = self.client.call("get", {"id": item["id"]})
                self._requests[request_id] = ("preview", item["id"])
        self.changed.emit()

    @Slot(str)
    def copy_item(self, item_id):
        request_id = self.client.call("copy", {"id": item_id})
        self._requests[request_id] = ("copy", item_id)

    @Slot(str, str)
    def copy_transformed(self, item_id, transform):
        request_id = self.client.call("copy", {"id": item_id, "transform": transform})
        self._requests[request_id] = ("copy", item_id)

    @Slot("QVariantList")
    def merge_copy(self, ids):
        if len(ids) < 2:
            return
        request_id = self.client.call("merge", {"ids": list(ids), "separator": "\n", "action": "copy"})
        self._requests[request_id] = ("merge", None)

    @Slot(str)
    def paste_item(self, item_id):
        # confirmed=true is an additive daemon extension; older daemons' queued
        # acknowledgments must not be mistaken for completed paste operations.
        request_id = self.client.call("paste", {"id": item_id, "confirmed": True, "target_window": self._target_window})
        self._requests[request_id] = ("paste", item_id)

    def set_target(self, window_id):
        self._target_window = window_id

    @Slot(str)
    def paste_plain(self, item_id):
        request_id = self.client.call("paste", {"id": item_id, "plain": True, "confirmed": True, "target_window": self._target_window})
        self._requests[request_id] = ("paste", item_id)

    @Slot()
    def clear_history(self):
        request_id = self.client.call("clear", {"include_pinned": False})
        self._requests[request_id] = ("refresh", None)

    @Slot(str, bool)
    def pin_item(self, item_id, pinned):
        request_id = self.client.call("pin", {"id": item_id, "pinned": pinned})
        self._requests[request_id] = ("refresh", None)

    @Slot(str)
    def delete_item(self, item_id):
        request_id = self.client.call("delete", {"id": item_id})
        self._requests[request_id] = ("refresh", None)

    def _response(self, request_id, result):
        action, value = self._requests.pop(request_id, (None, None))
        if not isinstance(result, dict):
            self._failed(request_id, "Invalid daemon response")
            return
        if action in ("query", "page"):
            items = result.get("items", [])
            if not isinstance(items, list) or not all(
                isinstance(item, dict) and isinstance(item.get("id"), str)
                and all(isinstance(item.get(key, ""), str) for key in ("preview", "content", "masked_content", "type", "thumbnail_path", "blob_path"))
                for item in items
            ) or type(result.get("has_more", False)) is not bool:
                self._failed(request_id, "Invalid daemon response")
                return
        if action == "query" and request_id == self._query_id:
            self._items = result.get("items", [])
            self._has_more = bool(result.get("has_more"))
            self._last_params = value
        elif action == "page" and value == (self._query_id, len(self._items)):
            self._items += result.get("items", [])
            self._has_more = bool(result.get("has_more"))
        elif action == "preview" and value == self._selected.get("id"):
            item = result.get("item", result)
            if not isinstance(item, dict) or not all(isinstance(item.get(key, ""), str) for key in ("content", "masked_content")):
                self._failed(request_id, "Invalid daemon response")
                return
            self._preview = item.get("masked_content", "Sensitive content hidden") if item.get("sensitive") else item.get("content", "")
        elif action == "copy" and result.get("success") is True:
            self.dismiss()
        elif action == "merge" and isinstance(result.get("text"), str):
            self.dismiss()
        elif action == "paste":
            if result.get("pasted") is True:
                self.dismiss()
            else:
                self._message = "Copied — paste with Ctrl+V. Automatic paste was not confirmed."
        elif action == "refresh":
            self.search(self._query, self._category)
        self.changed.emit()

    def _failed(self, request_id, message):
        self._requests.pop(request_id, None)
        self._message = message
        self.changed.emit()

    def _event(self, name, _payload):
        if name in ("item_added", "item_deleted", "item_updated") and self._visible:
            self.search(self._query, self._category)


class ServiceSetup(QThread):
    result = Signal(bool, str)

    def run(self):
        from .startup import enable_service
        try:
            enable_service()
            self.result.emit(True, "Startup enabled. Connecting…")
        except Exception as error:
            self.result.emit(False, f"Could not enable the user service: {error}. Install the companion service first or configure session autostart.")
