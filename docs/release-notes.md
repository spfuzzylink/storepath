# Storepath 0.2.0 — a storage decision you can demo

Explore block, object, and hybrid storage in a browser, then take the same evaluator into a CLI or Go application.

**[Open the live demo](https://spfuzzylink.github.io/storepath/)** · **[Download the offline HTML](https://github.com/spfuzzylink/storepath/releases/download/v0.2.0/storepath-demo.html)** · [Release assets](https://github.com/spfuzzylink/storepath/releases/tag/v0.2.0)

## What's new

- An interactive browser interface with six editable workload presets, compatible candidate comparisons, itemized modeled costs, and report export.
- The existing Go evaluator compiled to WebAssembly, so browser decisions use the same logic as the library and CLI.
- A self-contained HTML download for offline demos: no installation, server, cloud account, or credentials.
- A browser-first walkthrough, real application use cases, and a guide to using Storepath alongside provider pricing, infrastructure cost review, and production analytics tools.

The schema version 1 input and Go evaluation contract remain unchanged. Workload constraints, explicit rate cards, hybrid cache accounting, and latency evidence still govern each decision.

## Start with telemetry retention

Open the demo and inspect the default telemetry preset: $808.00/month on block versus $247.31/month for a compatible hybrid design. Its $560.69 modeled monthly difference comes from synthetic workload and price inputs. Change retention or origin traffic, inspect the cost components, and export the report.

These are illustrative estimates, not live provider quotes, achieved customer savings, total cost of ownership, or benchmark measurements.

## CLI and library

From a checkout of this tag:

```sh
sh scripts/install.sh
bin/storepath demo
```

The prebuilt CLI needs no Go installation and runs locally without network access. It also accepts `evaluate FILE` and returns structured JSON. Release archives cover macOS and Linux on ARM64 and x86-64, with SHA-256 checksums and build information.

Add the library to a Go module:

```sh
go get github.com/spfuzzylink/storepath@v0.2.0
```

See [installation](https://github.com/spfuzzylink/storepath/blob/v0.2.0/docs/installation.md) for download verification, source builds, and requirements.

## Release scope

This remains an experimental prerelease. Storepath does not provision infrastructure, move data, generate latency measurements, or establish availability and durability equivalence. Review the [cost model](https://github.com/spfuzzylink/storepath/blob/v0.2.0/docs/cost-model.md) and [architecture](https://github.com/spfuzzylink/storepath/blob/v0.2.0/docs/architecture.md) before applying it to a design decision.

Checksums verify download integrity and are not independent signatures. macOS executables are not Developer ID signed or notarized. There is no production availability or support SLA. See [security](https://github.com/spfuzzylink/storepath/blob/v0.2.0/SECURITY.md) for trust boundaries.
