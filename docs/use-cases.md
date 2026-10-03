# Six storage decisions to explore

Start with the problem closest to your workload. [Open the interactive demo](https://spfuzzylink.github.io/storepath/) to change the inputs and inspect the decision, or [download the single-file demo](https://github.com/spfuzzylink/storepath/releases/download/v0.2.0/storepath-demo.html) for a walkthrough without an internet connection after download.

| Your question | Start here | What to inspect |
| --- | --- | --- |
| Can my database use direct object APIs? | Transactional database | The write contract that makes object storage incompatible |
| Can we retain more metrics without keeping every byte on block storage? | Telemetry retention | Mutable state, sealed data, cache, and request cost |
| Where should immutable backups live? | Backup repository | Capacity cost and the restore traffic assumption |
| Do selective reads require block storage? | Analytics range reads | Range GETs and the number of billable operations |
| Could a cache make an object service cheaper? | Hot object service | Cache cost and explicitly supplied origin traffic |
| Which plan meets a tight latency budget? | Strict latency | Missing evidence before a recommendation can qualify |

For a repeatable command-line walkthrough, run `bin/storepath demo`; for a complete JSON decision, run `bin/storepath evaluate examples/NAME.json`. All six use the same Go evaluator as the browser.

These are synthetic design scenarios, not customer case studies. Fixture sizes, traffic, and rates are illustrative. The example files are the source of truth; outcomes are not measured latency, deployed architectures, or achieved savings.

## 1. Transactional database: preserve the write contract

**Customer need:** an application database updates pages in place and depends on a filesystem and synchronous WAL persistence.

**Fixture:** [transactional-db.json](../examples/transactional-db.json). Expected result: **block**.

Direct object APIs do not satisfy this unmodified application contract. An inexpensive object capacity rate must not override that rejection. The example has no separable immutable tier, so a hybrid label would not solve the incompatibility.

Before deploying, validate the database's actual durability and recovery requirements, sustained IOPS, I/O size, throughput, tail latency, filesystem behavior, and host limits. Block storage does not itself create a multi-region database or supply application-level replication.

Object-native databases and specialized object storage modes are outside this example; the conclusion is about the declared contract, not every database design.

## 2. Observability retention: split ingestion from sealed data

**Customer need:** accept an active stream of metrics while retaining older data for historical queries.

**Fixture:** [telemetry-retention.json](../examples/telemetry-retention.json). Expected result: **hybrid**.

The application can separate mutable ingestion state from immutable sealed blocks. Storepath prices the mutable head/WAL and explicit cache on block storage, and all immutable retained data on object storage. This resembles the storage boundaries documented by [Cortex](https://cortexmetrics.io/docs/blocks-storage/), whose [store gateway](https://cortexmetrics.io/docs/blocks-storage/store-gateway/) also keeps index headers locally.

**Demo walkthrough:** the preset includes 100 GiB of mutable state, 10,000 GiB of retained immutable data, and a 100 GiB hybrid cache. It models **$808.00/month for block** and **$247.31/month for hybrid**, a **$560.69 modeled monthly difference**. Direct object storage is incompatible with the declared mutable-write contract. The difference uses synthetic prices and excludes unmodeled costs; it is not realized savings or a TCO comparison.

Increase retention, then increase hybrid origin reads. Inspect which line items move and whether the recommendation changes. Export the report with your revised inputs to make the assumptions reviewable.

Measure ingestion recovery, block shipping, query range fan-out, compaction operations, label cardinality effects, and cold-cache behavior. Retained bytes do not imply a particular query latency. Cache capacity does not imply a particular hit ratio. Enter the hybrid's origin operations explicitly and include compaction and retry traffic.

## 3. Backup repository: immutable capacity with restore obligations

**Customer need:** retain immutable backup objects and retrieve them for occasional restores.

**Fixture:** [backup-repository.json](../examples/backup-repository.json). Expected result: **object** under its synthetic access and price assumptions.

Whole-object writes and reads fit the modeled object contract. The example demonstrates Standard-like object storage, not an archival class with delayed retrieval or minimum retention charges.

The next design test is recovery: validate restore time, throughput, integrity, permissions, and the independent metadata needed to locate backups. Include billable retrieval and egress if applicable. An inexpensive monthly capacity estimate says nothing about whether a restoration meets its recovery-time objective.

Sealed log archives and versioned ML checkpoints can use the same immutable-object pattern once their write is complete. Live log appends or partially written checkpoints have different semantics and should be modeled separately.

## 4. Analytics range reads: selective access can remain object-compatible

**Customer need:** read selected ranges from immutable analytical files without loading each whole file.

**Fixture:** [analytics-range-reads.json](../examples/analytics-range-reads.json). Expected result: **object** under its supplied costs.

Random read positions are compatible with [S3 byte-range GET](https://docs.aws.amazon.com/AmazonS3/latest/API/API_GetObject.html). They are not the same as random in-place writes. One GET cannot retrieve multiple disjoint ranges, so the object's layout and query engine determine request fan-out and bytes fetched.

Measure range size, request concurrency, metadata lookup traffic, retry behavior, and query amplification. Count actual object operations, not only end-user queries. Compression or file format may prevent an arbitrary range from being independently useful.

## 5. Hot object service: price the cache and the requests

**Customer need:** serve many repeated reads of immutable objects, where request costs can become significant relative to capacity cost.

**Fixture:** [hot-object-service.json](../examples/hot-object-service.json). Expected result: **hybrid** with the fixture's explicitly reduced origin traffic.

The hybrid estimate includes a block cache and the complete retained object tier. It does not remove cached bytes from object storage. Its lower origin request counts are supplied assumptions, not the output of a cache simulation.

The fixture assumes 100 GiB retained, one billion reads per month, a 20 GiB cache, and 50 million post-cache origin reads. Both block plans use three copies. The standalone block plan includes a synthetic $120/month additional serving cost; the hybrid block plan includes $30/month. With the illustrative rate card, the modeled totals are $144.00 block, $402.35 object, and $57.15 hybrid per month. These cost and cache-effectiveness assumptions are visible in the JSON and can reverse the ranking when changed. All six examples assume no billable egress; a real deployment must account for it on each candidate.

Measure the working set, request and byte hit rates, eviction behavior, cache-fill traffic, and cold-start impact. For sufficiently request-heavy tiny objects, a fully block-backed candidate may also have a lower modeled bill. That result needs a compatible serving layer, realistic compute/network costs, and appropriate availability design before it becomes a system recommendation.

When clients consume the same payload outside the deployment, account for delivery egress on every relevant candidate. A cache reduces origin reads, not necessarily bytes sent to customers.

## 6. Strict latency: expose the missing evidence

**Customer need:** satisfy a p99 latency budget while selecting a storage design.

**Fixture:** [strict-latency.json](../examples/strict-latency.json). Expected result: **benchmark_required** because the fixture supplies a target without the required measurements.

The evaluator can show compatibility and costs, but cannot infer tail latency from capacity prices or backend names. Supply `measured_p99_ms` for each eligible candidate after a representative experiment. With a target, only candidates with passing supplied evidence can satisfy it.

Use equivalent request mixes, data sizes, concurrency, and observation windows. Report cache state and examine failure/recovery behavior separately. A single favorable p99 number is not an availability commitment or proof of production readiness.

## Kubernetes and AI infrastructure workflows

### Kubernetes: decide the data contract before choosing a volume

| Application and end customer | Start with | What the comparison helps decide | Validate next |
| --- | --- | --- | --- |
| A StatefulSet database serving customer transactions | Transactional database | Whether the filesystem, in-place update, and WAL contract require a block-backed filesystem | CSI driver behavior, access modes, topology, failover, snapshots, backup and restore |
| A monitoring service retaining tenant metrics for incident investigation | Telemetry retention | How much active state/cache stays on block versus sealed history in object storage | Recovery, query fan-out, cardinality, cold-cache p99, and explicit origin traffic |
| An application backup service protecting tenant data | Backup repository | How retained immutable capacity and restore operations affect the estimate | Application-consistent snapshots, restore time, retention policy, and failure drills |

[Kubernetes PersistentVolumes](https://kubernetes.io/docs/concepts/storage/persistent-volumes/) distinguish filesystem and raw-block volume modes and expose provider-dependent access modes. This model's block candidate means a block volume with an application-compatible filesystem; it does not interpret a StorageClass, provision a PVC, verify multi-node access, or predict rescheduling behavior. Object access means the application can use object APIs directly. A bucket is not automatically a replacement for a mounted filesystem.

### AI: separate retained artifacts from the active working set

| Application and end customer | Start with | What to supply | Validate next |
| --- | --- | --- | --- |
| Training datasets read through object-native loaders | Analytics range reads | Immutable dataset size, range-request fan-out, retrieval and network traffic | Data-loader throughput, small-file overhead, parallelism, and GPU idle time |
| Sealed checkpoints retained for restart and reproducibility | Backup repository; telemetry retention if local mutable staging remains | Checkpoint capacity, upload/restore operations, staging footprint, and copy counts | Checkpoint completion/atomicity, restart time, upload overlap, and recovery correctness |
| Model weights or artifacts repeatedly loaded by serving workers | Hot object service | Object footprint, explicit block cache size, post-cache origin requests, serving compute and network costs | Cold starts, cache eviction, fleet rollout bursts, and end-to-end p99 |

For example, [SageMaker's checkpoint workflow](https://docs.aws.amazon.com/sagemaker/latest/dg/model-checkpoints.html) separates local checkpoints from S3 synchronization. That illustrates a lifecycle boundary worth modeling; it does not make this tool a SageMaker integration. If a training framework requires a POSIX or distributed filesystem, retain that constraint. [FSx for Lustre's S3 integration](https://docs.aws.amazon.com/fsx/latest/LustreGuide/fsx-data-repositories.html) is a distinct managed filesystem design outside this direct-object model.

These workflows reuse the shipped six scenarios and evaluator. Storepath estimates the explicit storage components and evaluates supplied latency evidence; it does not estimate GPU utilization, training speed, cache hit ratios, or checkpoint correctness. Use the exported input and decision to plan the workload-specific benchmark before changing production placement.

## Turning an example into a design decision

Start with an application's data contract, then document capacity and access estimates, use rates with a stated date and deployment context, and check compatibility. Inspect [cost line items and exclusions](cost-model.md), gather latency evidence where required, and review availability and operational requirements independently.

The useful outcome is an auditable reason to test a candidate architecture—not a promise that the cheapest synthetic example will be cheapest for every customer.
