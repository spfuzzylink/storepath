# Storepath 0.1.0 — storage decisions with explicit assumptions

Storepath is an experimental Go library and CLI for evaluating block versus object
storage against request semantics, access patterns, and modeled monthly cost.
It is a standalone public prototype using synthetic examples; it contains no
customer or employer data and makes no claim of production deployment or savings.

## Try it

From a checkout of this tag:

```sh
./scripts/install.sh
./bin/storepath demo
```

The prebuilt demo needs no Go installation, cloud account, or runtime network
access. Library users can add `github.com/spfuzzylink/storepath@v0.1.0` to a Go module.
The executable also accepts `evaluate FILE` and returns structured JSON.

## Included

- Explicit workload constraints and explainable block/object recommendations.
- Deterministic customer scenarios and transparent, configurable cost assumptions.
- A Go standard-library-only module and command-line interface.
- macOS and Linux binaries for ARM64 and x86-64, checksums, normalized archives,
  Go notices, architecture, use-case, cost-model, and installation documentation.
- Native packaged-executable checks on all four release targets, a Go 1.26
  compatibility check, and a source/history publication guard before release.
- An installer tested for bad downloads, checksum mismatches, malformed archives,
  unsafe members, and preservation of existing executables on failure.

Costs are estimates from caller-supplied or explicitly illustrative inputs.
They are not live price quotes, latency benchmarks, or realized customer savings.
The prototype does not provision infrastructure or move data. Validate workload
behavior and full provider pricing before committing to a storage design.

This is a prerelease with no production availability or support SLA. Checksums
are not independent signatures; macOS executables are not Developer ID signed
or notarized. See `SECURITY.md` in the source repository for the trust boundaries.
