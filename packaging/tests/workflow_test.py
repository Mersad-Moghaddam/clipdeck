from pathlib import Path
import unittest

ROOT = Path(__file__).resolve().parents[2]

class WorkflowTests(unittest.TestCase):
    def test_ci_uses_module_toolchain(self):
        ci = (ROOT / '.github/workflows/ci.yml').read_text()
        self.assertIn('go-version-file: go.mod', ci)
        self.assertIn('bash packaging/tests/install_test.sh', ci)

    def test_release_is_draft_and_tests_before_build(self):
        release = (ROOT / '.github/workflows/release.yml').read_text()
        self.assertIn('draft: true', release)
        self.assertIn('go test ./...', release)
        self.assertNotIn('nfpm@latest', release)

if __name__ == '__main__':
    unittest.main()
