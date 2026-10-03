"""Single-file browser packaging boundaries and escaping."""

import importlib.util
import json
from pathlib import Path
import tempfile
import unittest


spec = importlib.util.spec_from_file_location("build_demo", Path(__file__).with_name("build-demo.py"))
build = importlib.util.module_from_spec(spec)
spec.loader.exec_module(build)


class BrowserBuildTests(unittest.TestCase):
    def fixture(self, root):
        for name in build.SOURCES:
            path = root / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text("fixture\n")
        (root / "VERSION").write_text("0.2.0\n")
        for name in build.EXAMPLES:
            (root / "examples" / (name + ".json")).write_text(json.dumps({
                "name": "</script><script>danger()</script>&\u2028", "marker": "__STOREPATH_APP__",
            }))
        (root / "web/index.template.html").write_text("\n".join((
            "<style>__STOREPATH_STYLE__</style>",
            "<script>window.STOREPATH_PRESETS=__STOREPATH_PRESETS__;</script>",
            "<script>window.STOREPATH_VERSION='__STOREPATH_VERSION__';</script>",
            '<script id="storepath-wasm" type="application/octet-stream">__STOREPATH_WASM_BASE64__</script>',
            "<script>__STOREPATH_RUNTIME__</script>", "<script>__STOREPATH_BOOT__</script>",
            "<script>__STOREPATH_APP__</script>", "<pre>__STOREPATH_NOTICES__</pre>",
        )))
        (root / "LICENSE").write_text("A license with <literal> & terms")
        return root

    def test_single_file_preserves_data_and_notices_without_script_injection(self):
        with tempfile.TemporaryDirectory() as temp:
            root = self.fixture(Path(temp))
            first = build.assemble(root, b"\0asm\x01\0\0\0", "runtime()")
            self.assertEqual(first, build.assemble(root, b"\0asm\x01\0\0\0", "runtime()"))
            self.assertNotIn("<script>danger()", first)
            self.assertIn("\\u003c/script\\u003e", first)
            self.assertIn("__STOREPATH_APP__", first)  # Data was not substituted again.
            self.assertIn("A license with &lt;literal&gt; &amp; terms", first)
            self.assertIn("AGFzbQEAAAA=", first)

    def test_json_script_escaping_roundtrips_unicode_and_closing_tags(self):
        value = {"name": "</SCRIPT> & café \u2028 \u2029", "n": 1.5}
        encoded = build.script_json(value)
        self.assertNotIn("<", encoded)
        self.assertNotIn("&", encoded)
        self.assertEqual(json.loads(encoded), value)
        with self.assertRaises(ValueError):
            build.script_json({"invalid": float("nan")})

    def test_missing_duplicate_unknown_markers_fail_closed(self):
        with tempfile.TemporaryDirectory() as temp:
            root = self.fixture(Path(temp))
            path = root / "web/index.template.html"
            template = path.read_text()
            for altered in (template.replace("__STOREPATH_STYLE__", ""),
                            template + "__STOREPATH_STYLE__", template + "__STOREPATH_UNKNOWN__"):
                path.write_text(altered)
                with self.assertRaises(ValueError):
                    build.assemble(root, b"\0asm\x01\0\0\0", "runtime()")

    def test_inline_source_rejects_closing_script_and_style(self):
        for tag in ("script", "style"):
            for end in (">", " ", "/"):
                with self.assertRaises(ValueError):
                    build.raw_source("</" + tag.upper() + end, tag)

    def test_source_allowlist_and_symlinks_reject_local_files(self):
        with tempfile.TemporaryDirectory() as temp:
            root = self.fixture(Path(temp))
            for name in ("../other", ".env", "web/private.json", "examples/private.json"):
                with self.assertRaises(ValueError):
                    build.read_public(root, name)
            (root / "LICENSE").unlink()
            (root / "LICENSE").symlink_to("VERSION")
            with self.assertRaises(ValueError):
                build.read_public(root, "LICENSE")
            (root / "web").rename(root / "real-web")
            (root / "web").symlink_to("real-web", target_is_directory=True)
            with self.assertRaises(ValueError):
                build.read_public(root, "web/app.js")

    def test_invalid_version_and_wasm_cannot_be_packaged(self):
        with tempfile.TemporaryDirectory() as temp:
            root = self.fixture(Path(temp))
            with self.assertRaises(ValueError):
                build.assemble(root, b"not wasm", "runtime()")
            (root / "VERSION").write_text("0.2.0<script>")
            with self.assertRaises(ValueError):
                build.assemble(root, b"\0asm\x01\0\0\0", "runtime()")


if __name__ == "__main__":
    unittest.main()
