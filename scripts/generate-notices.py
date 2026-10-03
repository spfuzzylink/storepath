#!/usr/bin/env python3
"""Inventory the native and browser builds and preserve upstream notices."""

import argparse
import json
import os
from pathlib import Path
import re
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[1]
TARGETS = ("darwin/amd64", "darwin/arm64", "linux/amd64", "linux/arm64", "js/wasm")
NOTICE_NAME = re.compile(r"(?:LICEN[CS]E|COPYING|NOTICE|PATENTS)(?:[._-].*)?", re.I)


class NoticeError(Exception):
    pass


def parse_go_json(data):
    decoder = json.JSONDecoder()
    packages, offset = [], 0
    while offset < len(data):
        while offset < len(data) and data[offset].isspace():
            offset += 1
        if offset == len(data):
            break
        package, offset = decoder.raw_decode(data, offset)
        if not isinstance(package, dict):
            raise NoticeError("Go returned a non-object package")
        packages.append(package)
    return packages


def inspect_go(go):
    env = dict(os.environ, CGO_ENABLED="0", GOFLAGS="", GOWORK="off", GOEXPERIMENT="",
               GOAMD64="v1", GOARM64="v8.0")
    try:
        metadata = json.loads(subprocess.check_output(
            [go, "env", "-json", "GOROOT", "GOVERSION"], cwd=ROOT,
            env=env, text=True, stderr=subprocess.PIPE))
        graphs = {}
        for target in TARGETS:
            env["GOOS"], env["GOARCH"] = target.split("/")
            command = "./cmd/storepath-web" if target == "js/wasm" else "./cmd/storepath"
            data = subprocess.check_output(
                [go, "list", "-deps", "-json", "-mod=readonly", command],
                cwd=ROOT, env=env, text=True, stderr=subprocess.PIPE)
            graphs[target] = parse_go_json(data)
        return graphs, Path(metadata["GOROOT"]), metadata["GOVERSION"]
    except (OSError, ValueError, KeyError, subprocess.CalledProcessError) as error:
        raise NoticeError("cannot inspect production Go dependencies; check the Go toolchain") from error


def read_notice(path):
    if path.is_symlink() or not path.is_file():
        raise NoticeError("a required upstream notice is missing or symlinked")
    try:
        text = path.read_bytes().decode("utf-8")
    except (OSError, UnicodeDecodeError) as error:
        raise NoticeError("an upstream notice cannot be read as UTF-8") from error
    if not text.strip():
        raise NoticeError("an upstream notice is empty")
    return text


def render_notices(graphs, goroot, go_version):
    paths = {goroot / "LICENSE"}
    if (goroot / "PATENTS").is_file():
        paths.add(goroot / "PATENTS")
    for packages in graphs.values():
        for package in packages:
            if package.get("ForTest") or package.get("Error") or package.get("DepsErrors"):
                raise NoticeError("only complete production dependency graphs are supported")
            module = package.get("Module")
            if module and not module.get("Main"):
                raise NoticeError("external dependencies require a deliberate license and design review")
            if module and module.get("Replace"):
                raise NoticeError("replaced dependencies require a deliberate review")
            if not package.get("Standard") or not package.get("Dir"):
                continue
            directory = Path(package["Dir"])
            if directory != goroot and goroot not in directory.parents:
                raise NoticeError("a standard-library package is outside the toolchain")
            while True:
                if directory.is_symlink():
                    raise NoticeError("notice inputs must not follow symlinked directories")
                for path in directory.iterdir():
                    if NOTICE_NAME.fullmatch(path.name) and (path.is_file() or path.is_symlink()):
                        paths.add(path)
                if directory == goroot:
                    break
                directory = directory.parent
    lines = [
        "# Third-party notices\n\n",
        "Storepath's own source is MIT licensed; see `LICENSE`. Release and installer "
        "scripts adapt MIT-licensed code from [Questlock](https://github.com/spfuzzylink/questlock), "
        "Copyright (c) 2026 spfuzzylink contributors.\n\n",
        "The library and executables use only the Go standard library. There are no external Go modules. "
        "Prebuilt native and WebAssembly executables include Go runtime and standard-library code under the notices below. "
        "The browser demo also embeds `lib/wasm/wasm_exec.js` from the same Go toolchain, covered by the Go LICENSE below.\n\n",
        "Generated with `python3 scripts/generate-notices.py` from `go list -deps -json -mod=readonly` "
        "on `./cmd/storepath` and `./cmd/storepath-web` "
        "for all production targets with `CGO_ENABLED=0` and default build experiments. "
        "Upstream notice texts are preserved verbatim.\n\n",
        "Toolchain: `" + go_version + "`. Targets: " + ", ".join("`" + t + "`" for t in sorted(graphs)) + ".\n\n",
        "Regenerate and review this document when changing the toolchain, build configuration, "
        "or dependencies. It is a package-level inventory, not a function-level linking report.\n\n",
    ]
    for path in sorted(paths, key=lambda p: p.relative_to(goroot).as_posix()):
        text = read_notice(path)
        fence = "`" * max(3, max((len(x) for x in re.findall(r"`+", text)), default=0) + 1)
        lines.extend(["## Go `" + path.relative_to(goroot).as_posix() + "`\n\n",
                      fence + "text\n", text, "" if text.endswith("\n") else "\n", fence + "\n\n"])
    # Strip only generated Markdown spacing after the final closing fence;
    # upstream notice text remains unchanged inside that fence.
    return ("".join(lines).rstrip("\n") + "\n").encode("utf-8")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--go", default="go")
    parser.add_argument("--check", action="store_true")
    args = parser.parse_args()
    output = ROOT / "THIRD_PARTY_NOTICES.md"
    try:
        graphs, goroot, version = inspect_go(args.go)
        expected = render_notices(graphs, goroot, version)
        if args.check:
            if not output.is_file() or output.read_bytes() != expected:
                raise NoticeError("THIRD_PARTY_NOTICES.md is stale; use the pinned release toolchain to regenerate and review it")
            print("Third-party notices match all native/browser dependency graphs and toolchain.")
        else:
            output.write_bytes(expected)
            print("Generated THIRD_PARTY_NOTICES.md for all native/browser production targets.")
    except (NoticeError, OSError, KeyError) as error:
        message = str(error) if isinstance(error, NoticeError) else "cannot read production source metadata"
        print("Notice generation failed: " + message, file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
