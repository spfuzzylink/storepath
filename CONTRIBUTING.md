# Contributing to Storepath

Storepath is an experimental, local decision-support library for choosing block
or object storage from workload constraints and explicit cost assumptions.
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
four native platform jobs, the Go 1.26 compatibility job, and public-source/history
checks succeed. No release or archive can prove the accuracy of workload inputs.

Contributions use the repository's [MIT license](LICENSE). Preserve upstream
attributions. Report security concerns using [SECURITY.md](SECURITY.md).
