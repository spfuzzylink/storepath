# Contributing to Storepath

Storepath is deprecated. Please direct new contributions to [BlockOrBucket](https://github.com/spfuzzylink/blockorbucket). The instructions below describe the retained source.

Storepath is an experimental, local decision-support library for choosing block
object, or hybrid storage from workload constraints and explicit cost assumptions.
Start with the [use cases](docs/use-cases.md), [architecture](docs/architecture.md),
and [cost model](docs/cost-model.md). Use synthetic data in examples and reports.

Useful contributions include a missing decision boundary, an incorrect cost
formula with a reproducible example, or clearer guidance on when an estimate is
insufficient. Open an issue before substantial features or external dependencies.
Do not present synthetic estimates as measured customer savings or benchmarks.

## Development

Use Go 1.26 or newer, Python 3.10 or newer, Git, and a C compiler for the Go race
detector. From a full clone:

```sh
make check
make demo
```

For the browser demo, use the pinned release Go toolchain and Node.js 24 or newer:

```sh
make browser-demo
make test-demo
```

Open `dist/storepath-demo.html` directly in a browser. UI sources live in `web/`;
the WebAssembly adapter in `cmd/storepath-web` calls the same strict decoder and
evaluator as the CLI. Keep pricing and compatibility logic in the Go library.

The release also exercises the actual interface offline in Chromium. Playwright
is a development dependency only; it is never included in the downloadable demo:

```sh
npm ci
npx playwright install chromium
npm run test:ui
```

Add focused regression tests for behavior changes. Explain the problem, resulting
behavior, and validation in a pull request. Keep workload semantics and pricing
inputs explicit; do not add cloud credentials or live provider access to the
runtime. The runtime must remain usable without network access or a cloud account.

Only intended public source and documentation belong in Git. Keep local input,
CVs, employer/customer data, credentials, environment files, and absolute local
paths outside the repository. The publication guard checks the index, tracked
working files, and all reachable history. Configure your GitHub noreply address
for public commits; ignored files are not a substitute for review.

## Releases

Release builds use Go 1.27.1 with CGO disabled. Changes to Go, targets, imports, or
build flags require regenerating and reviewing the standard-library notices:

```sh
python3 scripts/generate-notices.py
python3 scripts/package.py
```

Review the exact source, notices, archive contents, and checksums before tagging.
The tag must match `VERSION`. CI publishes an explicit prerelease only after all
four native platform jobs, the Go 1.26 compatibility job, the embedded browser
engine parity checks, and public-source/history checks succeed. The same gates
protect the hosted demo. No release or archive can prove the accuracy of workload inputs.

Contributions use the repository's [MIT license](LICENSE). Preserve upstream
attributions. Report security concerns using [SECURITY.md](SECURITY.md).
