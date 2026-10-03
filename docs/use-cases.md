# Customer use cases

These are synthetic design scenarios, not customer case studies. Fixture sizes, traffic, prices, and outcomes demonstrate the evaluator. They are not measured latency, deployed architectures, or achieved savings.

Run `bin/storepath demo` for the six cases or `bin/storepath evaluate examples/NAME.json` for one complete JSON decision. The example files are the source of truth for quantities and rates.

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

## Turning an example into a design decision

Start with an application's data contract, then document capacity and access estimates, use rates with a stated date and deployment context, and check compatibility. Inspect [cost line items and exclusions](cost-model.md), gather latency evidence where required, and review availability and operational requirements independently.

The useful outcome is an auditable reason to test a candidate architecture—not a promise that the cheapest synthetic example will be cheapest for every customer.
