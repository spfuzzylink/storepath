# Architecture

Storepath is a deterministic evaluator. It maps workload requirements and an explicit rate card to eligible storage plans, cost line items, and a recommendation status. It performs no storage I/O and makes no network calls.

The reusable API is `storepath.Evaluate(Input) (Decision, error)`. The CLI decodes JSON, calls that API, and renders the result. `evaluate FILE` handles one input; `demo` evaluates the bundled synthetic cases. `demo --json` makes those results machine-readable.

## Input contract

The top-level document contains `schema_version: 1`, `workload`, and `rates`.

`workload` describes:

- Logical capacity: `mutable_gib` and `immutable_gib`.
- Mutation mode: `immutable`, `replace_object`, `random_in_place`, or `append_and_seal`.
- Application requirements: `requires_posix`, `requires_fsync`, and `can_split_tiers`.
- A `block` plan and `object_access` estimates.
- An optional explicit `hybrid` plan, containing `cache_gib`, its own `block` plan, and its own `object_access` estimates.
- Optional `target_p99_ms` and `measured_p99_ms` values keyed by `block`, `object`, or `hybrid`.

A block plan supplies its copy count, capacity headroom per copy, provisioned IOPS and MiB/s per copy, and any explicit additional monthly cost. Object access supplies GET, PUT, LIST, retrieval, egress, and additional monthly cost estimates. Hybrid object access is already the assumed post-cache traffic: Storepath does not invent a hit ratio or derive one from cache size.

The rate card includes its label, currency, date, illustrative flag, and component rates. See [the cost model](cost-model.md) and [complete fixtures](../examples) for units and examples.

## Evaluation order

### 1. Validate the input

Schema version, required values, supported mutation modes, finite nonnegative quantities, and copy counts are checked before evaluation. Total logical capacity must be positive. An `immutable` mutation mode requires zero mutable capacity; the other mutation modes require a positive mutable working set. A hybrid cache cannot exceed the immutable footprint. Supplied latency measurements must be positive and finite; an omitted or zero target means no latency budget is applied. Unknown JSON fields and duplicate keys are rejected by the CLI's strict decoder.

An error means the input cannot be evaluated. A valid input that lacks latency evidence is different: it produces a decision with a corresponding status.

JSON profiles must explicitly include every semantic flag, rate, and access quantity, even when the value is `false` or zero. Only `hybrid`, `target_p99_ms`, and `measured_p99_ms` are optional. This prevents an omitted rate from silently becoming free storage, or an omitted filesystem requirement from becoming an object-compatible application. Use canonical lowercase field names from the fixtures. The decoder also rejects null values, inputs larger than 2 MiB, and nesting deeper than 32 levels. Programmatic Go callers can deliberately use zero-valued fields and call `Validate` or `Evaluate` directly.

### 2. Check application semantics

Block storage is evaluated as storage beneath an application-compatible filesystem. This does not supply database transactions, multi-writer coordination, or a distributed filesystem automatically.

Direct object-only placement is ineligible if any of these are true:

- `requires_posix`;
- `requires_fsync`;
- mutation is `random_in_place`;
- mutation is `append_and_seal`.

`replace_object` is permitted: replacing an entire object is not equivalent to changing bytes in place. These gates apply to the supplied application contract and the modeled direct object API, not every possible object-backed application.

Hybrid placement requires an explicit plan, `can_split_tiers`, positive immutable capacity, and a positive mutable working set or cache. Its block tier contains mutable data plus cache; its object tier retains all immutable data. When the application requires mutable or filesystem operations, a read cache alone does not satisfy them: the plan needs an explicitly sized mutable block working set. The caller must ensure the application can implement that split and that the declared cache is useful.

### 3. Calculate modeled costs

For each eligible plan, calculate the component costs with full precision. Only display values are rounded. Ineligible plans cannot win because of a low rate card.

The block plan prices all logical data on block storage. The object plan prices all logical data in object storage. The hybrid plan prices its mutable data and cache on block storage and its complete immutable data on object storage.

### 4. Apply latency evidence

Without a latency target, the lowest modeled cost among eligible candidates is selected and the status is `modeled`.

With a target, only eligible candidates with supplied p99 evidence at or below the target can satisfy it. If any pass, choose the least expensive passing plan and return `latency_evidence_satisfied`. A missing measurement must never be treated as a zero-millisecond response. If none pass and at least one eligible candidate is unmeasured, return `benchmark_required`. If every eligible candidate has a failing measurement, return `no_feasible_plan`. These two statuses omit a recommendation.

Measurements are user inputs. Storepath does not verify their provenance or whether the test covered representative concurrency, tail behavior, cache misses, failover, or sustained load. A passing input is evidence for the modeled comparison, not an SLA guarantee.

## Output statuses

| Status | Interpretation |
| --- | --- |
| `modeled` | A candidate leads on the modeled cost components; no latency target was imposed. |
| `latency_evidence_satisfied` | The selected eligible candidate has supplied p99 evidence within the configured target. |
| `benchmark_required` | Missing performance evidence prevents a defensible conclusion against the target. |
| `no_feasible_plan` | No candidate meets the applicable requirements and available evidence. |

Consumers should inspect status, compatibility reasons, cost line items, and assumptions together. A candidate name alone is not sufficient to automate a migration.

## Scope boundaries

S3's object operations are [strongly consistent](https://docs.aws.amazon.com/AmazonS3/latest/userguide/Welcome.html); it also supports [byte-range reads](https://docs.aws.amazon.com/AmazonS3/latest/API/API_GetObject.html). Neither requirement alone excludes object storage. Disjoint ranges require separate requests, which belong in the access estimate.

This evaluator does not model [S3 Express append](https://docs.aws.amazon.com/AmazonS3/latest/userguide/directory-buckets-objects-append.html), [S3 Files](https://docs.aws.amazon.com/AmazonS3/latest/userguide/s3-files.html), or specialized database storage engines. Those products and architectures have different semantics and economics. See [Mountpoint's filesystem semantics](https://github.com/awslabs/mountpoint-s3/blob/main/doc/SEMANTICS.md) for why a filesystem interface does not necessarily imply a complete POSIX contract.

The hybrid observability pattern is informed by [Cortex blocks storage](https://cortexmetrics.io/docs/blocks-storage/) and its [store gateway](https://cortexmetrics.io/docs/blocks-storage/store-gateway/). Storepath neither implements Cortex nor predicts its query performance.
