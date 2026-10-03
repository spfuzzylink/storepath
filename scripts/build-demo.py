#!/usr/bin/env python3
"""Build an offline, single-file browser demo from explicit public inputs."""

import argparse
import base64
import html
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile


ROOT = Path(__file__).resolve().parents[1]
EXAMPLES = (
    "transactional-db", "telemetry-retention", "backup-repository",
    "analytics-range-reads", "hot-object-service", "strict-latency",
)
SOURCES = (
    "web/index.template.html", "web/style.css", "web/app.js", "web/boot.js",
    "LICENSE", "THIRD_PARTY_NOTICES.md", "VERSION",
) + tuple("examples/" + name + ".json" for name in EXAMPLES)
MARKER = re.compile(r"__STOREPATH_[A-Z0-9_]+__")


def read_public(root, name):
    """Do not follow even an allowlisted source through an external symlink."""
    if name not in SOURCES:
        raise ValueError("browser input is outside the public-file allowlist")
    path = root
    for part in Path(name).parts:
        path = path / part
        if path.is_symlink():
            raise ValueError("browser inputs must not be symlinks")
    return path.read_text(encoding="utf-8")


def script_json(value):
    """Preserve strings without allowing JSON data to close an HTML script."""
    return (json.dumps(value, ensure_ascii=True, allow_nan=False, sort_keys=True,
                       separators=(",", ":"))
            .replace("<", "\\u003c").replace(">", "\\u003e").replace("&", "\\u0026"))


def raw_source(source, tag):
    # These are code inputs, not user data. Reject ambiguous HTML parsing rather
    # than rewriting JavaScript/CSS and potentially changing its meaning.
    if re.search(r"</" + tag + r"(?:[\s>/])", source, flags=re.IGNORECASE):
        raise ValueError("inline source contains a closing HTML tag")
    return source


def assemble(root, wasm, runtime):
    if not wasm.startswith(b"\x00asm\x01\x00\x00\x00"):
        raise ValueError("browser engine is not a WebAssembly 1 module")
    version = read_public(root, "VERSION").strip()
    if not re.fullmatch(r"[0-9]+\.[0-9]+\.[0-9]+", version):
        raise ValueError("VERSION must contain a plain semantic version")
    presets = {name: json.loads(read_public(root, "examples/" + name + ".json"))
               for name in EXAMPLES}
    notices = read_public(root, "LICENSE") + "\n\n" + read_public(root, "THIRD_PARTY_NOTICES.md")
    values = {
        "__STOREPATH_STYLE__": raw_source(read_public(root, "web/style.css"), "style"),
        "__STOREPATH_APP__": raw_source(read_public(root, "web/app.js"), "script"),
        "__STOREPATH_RUNTIME__": raw_source(runtime, "script"),
        "__STOREPATH_BOOT__": raw_source(read_public(root, "web/boot.js"), "script"),
        "__STOREPATH_WASM_BASE64__": base64.b64encode(wasm).decode("ascii"),
        "__STOREPATH_PRESETS__": script_json(presets),
        "__STOREPATH_VERSION__": version,
        "__STOREPATH_NOTICES__": html.escape(notices),
    }
    template = read_public(root, "web/index.template.html")
    markers = MARKER.findall(template)
    if sorted(markers) != sorted(values):
        raise ValueError("template must include each expected browser marker exactly once")
    # One pass: marker-like workload text must remain data, never a second pass.
    return MARKER.sub(lambda match: values[match.group()], template)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, default=ROOT / "dist",
                        help="directory for storepath-demo.html and site/index.html")
    args = parser.parse_args()
    subprocess.run([sys.executable, str(ROOT / "scripts/generate-notices.py"), "--check"],
                   cwd=ROOT, check=True)
    goroot = Path(subprocess.check_output(["go", "env", "GOROOT"], cwd=ROOT, text=True).strip())
    runtime = (goroot / "lib/wasm/wasm_exec.js").read_text(encoding="utf-8")
    with tempfile.TemporaryDirectory(prefix="storepath-browser-") as temp:
        engine = Path(temp) / "storepath.wasm"
        env = dict(os.environ, GOOS="js", GOARCH="wasm", CGO_ENABLED="0",
                   GOFLAGS="", GOWORK="off", GOEXPERIMENT="")
        subprocess.run(["go", "build", "-mod=readonly", "-trimpath", "-buildvcs=false",
                        "-ldflags", "-s -w", "-o", str(engine), "./cmd/storepath-web"],
                       cwd=ROOT, env=env, check=True)
        document = assemble(ROOT, engine.read_bytes(), runtime)
    args.output.mkdir(parents=True, exist_ok=True)
    destination = args.output / "storepath-demo.html"
    with destination.open("w", encoding="utf-8", newline="\n") as stream:
        stream.write(document)
    site = args.output / "site"
    site.mkdir(parents=True, exist_ok=True)
    (site / "index.html").write_bytes(destination.read_bytes())
    print(f"Built {destination.name}: {destination.stat().st_size:,} bytes; site/index.html is identical")


if __name__ == "__main__":
    main()
