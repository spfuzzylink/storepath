# Install and try Storepath

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
sha256sum storepath_0.1.0_linux_amd64.tar.gz
```

On macOS use `shasum -a 256` and your platform's archive filename. Compare the result
with that exact filename's entry in `checksums.txt`, then extract and run:

```sh
tar -xzf storepath_0.1.0_linux_amd64.tar.gz
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
go get github.com/spfuzzylink/storepath@v0.1.0
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
./scripts/install.sh --version v0.1.0 --dir ./bin
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

A matching `vX.Y.Z` tag publishes an explicit experimental release only after the
four matching native platform jobs, Go 1.26 compatibility job, and full-history
publication guard pass. Release jobs publish the same archives exercised in CI.
