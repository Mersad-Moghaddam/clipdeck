import tempfile
import unittest
from pathlib import Path
from PySide6.QtCore import QCoreApplication
from clipdeck_ui.instance import InstanceServer

APP = QCoreApplication.instance() or QCoreApplication([])


class InstanceTests(unittest.TestCase):
    def test_second_launch_activates_existing_popup(self):
        with tempfile.TemporaryDirectory() as directory:
            path = str(Path(directory) / "ui.sock")
            first = InstanceServer(path)
            self.assertTrue(first.listen())
            targets = []
            first.activated.connect(targets.append)
            second = InstanceServer(path)
            self.assertTrue(second.activate("42"))
            for _ in range(10):
                APP.processEvents()
            self.assertEqual(targets, ["42"])
            first.close()
