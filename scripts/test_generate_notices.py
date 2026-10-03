"""Protect dependency boundaries and exact upstream notice text."""

import importlib.util
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location("notices", Path(__file__).with_name("generate-notices.py"))
notices = importlib.util.module_from_spec(spec)
spec.loader.exec_module(notices)


class NoticeTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="storepath-notices-test-")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.goroot = self.root / "toolchain"
        self.goroot.mkdir()
        (self.goroot / "LICENSE").write_bytes(b"Go license fixture\r\nCopyright fixture authors\r\n")

    def test_consecutive_go_json_is_parsed(self):
        self.assertEqual(notices.parse_go_json('\n{"Standard": true}\n\n{"Module": {"Main": true}}\n'),
                         [{"Standard": True}, {"Module": {"Main": True}}])
        with self.assertRaises(notices.NoticeError):
            notices.parse_go_json('[]')

    def test_notices_preserve_bytes_without_host_metadata(self):
        vendor = self.goroot / "src/vendor/example.org/lib"
        vendor.mkdir(parents=True)
        (vendor / "LICENSE").write_text("Vendored license fixture\n")
        package = {"Standard": True, "Dir": str(vendor)}
        graphs = {"linux/amd64": [package], "darwin/arm64": [package]}
        result = notices.render_notices(graphs, self.goroot, "go1.27.1")
        self.assertIn((self.goroot / "LICENSE").read_bytes(), result)
        self.assertTrue(result.endswith(b"\n"))
        self.assertFalse(result.endswith(b"\n\n"))
        self.assertEqual(result.count(b"Vendored license fixture"), 1)
        self.assertNotIn(str(self.root).encode(), result)
        self.assertEqual(result, notices.render_notices(dict(reversed(list(graphs.items()))), self.goroot, "go1.27.1"))

    def test_external_replaced_and_incomplete_graphs_fail_closed(self):
        for package in ({"Module": {"Path": "example.org/dependency", "Version": "v1.0.0"}},
                        {"Module": {"Main": True, "Replace": {"Path": "../local"}}},
                        {"ForTest": "example.org/pkg"}, {"Error": "invalid package"}):
            with self.subTest(package=package), self.assertRaises(notices.NoticeError):
                notices.render_notices({"linux/amd64": [package]}, self.goroot, "go1.27.1")

    def test_missing_or_symlinked_license_fails_closed(self):
        license_path = self.goroot / "LICENSE"
        license_path.unlink()
        with self.assertRaises(notices.NoticeError):
            notices.render_notices({}, self.goroot, "go1.27.1")
        outside = self.root / "outside"
        outside.write_text("not part of the toolchain")
        license_path.symlink_to(outside)
        with self.assertRaises(notices.NoticeError):
            notices.render_notices({}, self.goroot, "go1.27.1")

    def test_standard_package_outside_toolchain_is_rejected(self):
        with self.assertRaises(notices.NoticeError):
            notices.render_notices({"linux/amd64": [{"Standard": True, "Dir": str(self.root)}]}, self.goroot, "go1.27.1")


if __name__ == "__main__":
    unittest.main()
