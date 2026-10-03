# Install and try Storepath

## Start in a browser

**[Open the interactive demo](https://spfuzzylink.github.io/storepath/)** for an immediate walkthrough. Choose one of six presets, edit the workload, inspect compatible storage options and cost components, and export a report.

For a portable demo:

1. [Download `storepath-demo.html`](https://github.com/spfuzzylink/storepath/releases/download/v0.2.0/storepath-demo.html).
2. Open the downloaded file in a modern browser with WebAssembly enabled.
3. Explore **Telemetry retention**, then switch to **Strict latency** to see the evidence gate.

The file embeds the Go evaluator, WebAssembly runtime, examples, styles, and interface. It needs no installation, local server, cloud account, or credentials. After download, evaluation works offline and inputs stay in the browser. The interface evaluates the same Go library used by the CLI; it does not provision infrastructure or contact a cloud provider.

The release includes a SHA-256 checksum for the HTML file. If you verify downloads, compare `shasum -a 256 storepath-demo.html` on macOS or `sha256sum storepath-demo.html` on Linux with its entry in `checksums.txt` from [v0.2.0](https://github.com/spfuzzylink/storepath/releases/tag/v0.2.0).

## Prebuilt executable from a checkout

On macOS or Linux:

```sh
git clone https://github.com/spfuzzylink/storepath.git
cd storepath
./scripts/install.sh
./bin/storepath version
./bin/storepath demo
```

The installer selects your OS and CPU, downloads the version in `VERSION`, checks
its SHA-256 digest, and installs one executable into the checkout's ignored `bin/`
directory. It requires `curl`, `tar`, and either `sha256sum` or `shasum`. It requires
no Go installation, Python, cloud account, credentials, sudo, or shell changes.
The installer downloads from GitHub; the installed program runs locally without
network access. It installs the tagged release, not uncommitted checkout edits.

`demo` evaluates deterministic synthetic customer scenarios and prints decisions
and estimated cost components. These are modeled examples, not observed customer
results, benchmark measurements, or provider quotes. See the
[cost model](cost-model.md) before using an estimate in a design decision.

## Download an archive

Choose the matching archive and `checksums.txt` from
[Releases](https://github.com/spfuzzylink/storepath/releases):

| Platform | Archive suffix |
| --- | --- |
| macOS, Apple Silicon | `darwin_arm64.tar.gz` |
| macOS, Intel | `darwin_amd64.tar.gz` |
| Linux, ARM64 | `linux_arm64.tar.gz` |
| Linux, x86-64 | `linux_amd64.tar.gz` |

Prebuilt macOS binaries require macOS 13 or newer. Native Windows packages are not
provided. WSL can use Linux builds but is not a separately validated platform.

For Linux x86-64, verify the downloaded archive before extracting:

```sh
sha256sum storepath_0.2.0_linux_amd64.tar.gz
```

On macOS use `shasum -a 256` and your platform's archive filename. Compare the result
with that exact filename's entry in `checksums.txt`, then extract and run:

```sh
tar -xzf storepath_0.2.0_linux_amd64.tar.gz
./storepath version
./storepath demo
```

Each archive includes the executable, MIT license, Go notices, selected guides,
version, and `BUILDINFO.json` containing the source commit and target. Archive
ownership and timestamps are normalized. Archives contain no private inputs or
credentials. Checksums are not independent signatures; macOS binaries are not
Developer ID signed or notarized. Follow your device's software policy.

## Build from source or use the Go library

Install [Go 1.26 or newer](https://go.dev/doc/install), then in the checkout:

```sh
make demo
```

Or build directly, stamping the version:

```sh
go build -trimpath -ldflags "-X main.version=$(cat VERSION)" -o bin/storepath ./cmd/storepath
./bin/storepath help
./bin/storepath evaluate examples/telemetry-retention.json
```

To add the library to an existing Go module:

```sh
go get github.com/spfuzzylink/storepath@v0.2.0
```

Import `github.com/spfuzzylink/storepath`; see the README for the API and runnable
library example, and [architecture](architecture.md) for the decision model. The project has no
external Go dependencies. Initial Go installation, module retrieval, or automatic
toolchain download can use the network; the library and executable do not.

Run `make check` for formatting, Python installer/archive/guard tests, source and
history review, Go vet, and race tests. Development checks require Python 3.10+,
Git, and a C compiler for the Go race detector.

## Updates and removal

From an updated checkout, rerun `./scripts/install.sh`. An explicit release and
destination can be selected with:

```sh
./scripts/install.sh --version v0.2.0 --dir ./bin
```

The installer stages on the destination filesystem, preserves an existing binary
on failure, and atomically replaces it on success. It does not add anything to
`PATH`. To remove it, delete the installed `storepath` executable. No background
service or runtime state directory is created. Keep your input files separately.

## Maintainer packaging

Release builds are pinned to Go 1.27.1. Run
`python3 scripts/generate-notices.py` when updating the toolchain or imports, review
the resulting notices, and use `python3 scripts/package.py` to build all four
archives. Repeat `--target` to select particular targets. Outputs go into ignored
`dist/`. The builder verifies notices, uses an exact document allowlist, disables
CGO, strips build paths, and embeds the release version.

Run `make browser-demo` to produce `dist/storepath-demo.html` and an identical
`dist/site/index.html`. The builder embeds the Go WebAssembly engine, matching Go
runtime, UI, examples, and licenses into one file. `make test-demo` rebuilds it
and checks the actual embedded engine against the native CLI. Browser artifact
builds use Go 1.27.1; the parity check uses Node.js 24 in CI. Running the finished
HTML requires neither tool.

A matching `vX.Y.Z` tag publishes an explicit experimental release only after
four native platform jobs, Go 1.26 compatibility, embedded-browser-engine checks,
and the full-history publication guard pass. Release jobs publish the same native
archives and HTML exercised in CI, with a combined checksum manifest. On `main`,
the same required checks gate publication of the built site to GitHub Pages.
