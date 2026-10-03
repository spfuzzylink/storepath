# Storepath

**Explain block, object, and hybrid storage choices before provisioning.**

Storepath is an explainable Go library and CLI for evaluating storage designs. Give it a workload, explicit access estimates, and a rate card. It rejects incompatible plans, itemizes modeled monthly costs, and checks supplied latency evidence against your target.

Use it to answer concrete design questions:

- Does a database's filesystem and WAL contract rule out direct object storage?
- Should an observability platform keep its mutable head on block storage and sealed blocks in object storage?
- Do request charges change the economics of serving many tiny objects?
- Is an apparently inexpensive plan supported by latency evidence?

The CLI runs locally with no cloud credentials, network calls, or infrastructure changes. This is a decision PoC, not a storage engine, benchmark, cloud price feed, or total-cost calculator.

## Try the examples

```sh
git clone https://github.com/spfuzzylink/storepath.git
cd storepath
sh scripts/install.sh
bin/storepath demo
```

The installer downloads the pinned release for macOS or Linux and verifies its SHA-256 checksum. No Go installation is needed to run the CLI. See [installation](docs/installation.md) for requirements and source builds, and [release notes](docs/release-notes.md) for the experimental release's scope.

```sh
bin/storepath evaluate examples/telemetry-retention.json
bin/storepath demo --json
bin/storepath help
bin/storepath version
```

`evaluate FILE` emits a JSON decision. Invalid input returns a nonzero exit code. Unknown fields and duplicate JSON keys are rejected so a misspelled constraint cannot quietly disappear.

## What the examples demonstrate

All fixture workloads and prices are synthetic. Their results are modeled outcomes, not customer measurements or achieved savings.

| Example | Design lesson | Expected result |
| --- | --- | --- |
| [Transactional database](examples/transactional-db.json) | Random in-place writes and a WAL/filesystem contract must pass before cost matters. | Block |
| [Telemetry retention](examples/telemetry-retention.json) | Keep mutable ingestion on block storage and sealed data in object storage. | Hybrid |
| [Backup repository](examples/backup-repository.json) | Immutable, infrequently restored data is compatible with direct object APIs. | Object |
| [Analytics range reads](examples/analytics-range-reads.json) | Random reads do not imply random writes; object APIs support byte ranges. | Object |
| [Hot object service](examples/hot-object-service.json) | An explicit cache plan can reduce origin requests, but the cache also costs money. | Hybrid |
| [Strict latency target](examples/strict-latency.json) | A target without measurements is an unanswered question. | Benchmark required |

See [customer use cases](docs/use-cases.md) for the assumptions and follow-up measurements behind each example.

## Use the library

```sh
go get github.com/spfuzzylink/storepath@v0.1.0
```

The public entry point is:

```go
decision, err := storepath.Evaluate(input)
```

`input` is a `storepath.Input` with schema version 1, a workload, and a rate card. The library returns a structured decision or a validation error. A runnable example is included:

```sh
go run ./examples/library
```

Use [the input fixtures](examples) as complete JSON examples. [Architecture](docs/architecture.md) explains the evaluation order and statuses; [the cost model](docs/cost-model.md) defines every billed component.

## What makes a recommendation defensible

**Semantics first.** Direct object storage is rejected when the supplied application requires POSIX, fsync, random in-place updates, or an unsealed append stream. A split mutable/immutable design can still qualify for hybrid placement.

**Latency needs evidence.** A configured p99 target can be satisfied only by a supplied, passing measurement for an eligible plan. Storepath does not generate latency measurements or treat a low price as evidence of performance.

**Costs stay visible.** Capacity, provisioned block IOPS/throughput, object operations, retrieval, egress, and explicit additional monthly costs are itemized. A hybrid cache duplicates retained object data; its capacity is never deducted from the object tier.

**Scope stays explicit.** The comparison is a block volume plus an application-compatible filesystem versus direct general-purpose object APIs with S3 Standard-like semantics. Managed file layers, S3 Express, archive classes, object-native database engines, and distributed coordination are outside this PoC. A block-copy count does not establish availability or durability equivalence.

S3 supports both [strong object consistency](https://docs.aws.amazon.com/AmazonS3/latest/userguide/Welcome.html) and [range GETs](https://docs.aws.amazon.com/AmazonS3/latest/API/API_GetObject.html). Storepath does not reject it because of outdated assumptions about either.

## Apply it to your workload

Start with a fixture, replace its synthetic sizes, operation counts, and rates, and document the assumptions behind your inputs. For a latency-sensitive decision, measure each candidate with representative concurrency, request sizes, cache state, and failure conditions before supplying its p99.

Cost output covers the modeled components only. Compute, application replication, backups, KMS, CDN, operational effort, and other unmodeled costs need separate analysis or explicit additional-cost inputs. Read the [comparison limits](docs/cost-model.md#comparison-limits) before interpreting a difference as a potential saving.

Contributions that add reproducible workload cases, improve validation, or make assumptions easier to audit are welcome. Keep examples synthetic and omit credentials or customer data.
