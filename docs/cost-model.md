# Cost model

Storepath computes a monthly estimate for explicitly modeled components. The included rate card is illustrative; it is not a live provider quote, a regional price guarantee, or evidence of production savings.

All capacity quantities use GiB and throughput uses MiB/s. Currency is supplied in the rate card; no currency conversion occurs. When entering provider prices, normalize their billing units before use. Access quantities represent totals for the modeled month, not rates per second.

## Block storage

For one block candidate, let:

- `D` = logical GiB stored on block;
- `H` = `headroom_gib` per copy;
- `N` = `copies`;
- `I` = `provisioned_iops` per copy;
- `T` = `provisioned_mibps` per copy.

Then:

```text
capacity_cost   = N × (D + H) × block_gib_month
iops_cost       = N × max(0, I − block_included_iops) × block_iops_month
throughput_cost = N × max(0, T − block_included_mibps) × block_mibps_month
block_total     = capacity_cost + iops_cost + throughput_cost + extra_monthly_cost
```

`extra_monthly_cost` is a total for that candidate's block plan, added once rather than multiplied by copies. Use it for explicitly estimated costs not represented by a dedicated field, such as block-serving egress or replication transfer. Document what it includes and avoid counting those costs twice.

For the block-only candidate, `D = mutable_gib + immutable_gib`. For the hybrid candidate, `D = mutable_gib + cache_gib` using the hybrid's own block plan.

Provisioned IOPS and throughput are independent inputs. The calculator does not infer required IOPS from object request counts, validate provider-specific volume limits, model I/O splitting, or validate host bandwidth. Those are separate sizing tasks. AWS documents how [EBS I/O size affects IOPS and throughput](https://docs.aws.amazon.com/ebs/latest/userguide/ebs-io-characteristics.html). The included-baseline structure follows the shape of [gp3 pricing](https://aws.amazon.com/ebs/pricing/), while the input rate card controls the actual calculation.

## Object storage

For object-stored GiB `S` and the candidate's `object_access`:

```text
storage_cost   = S × object_gib_month
get_cost       = get_requests  / 1000 × get_per_1000
put_cost       = put_requests  / 1000 × put_per_1000
list_cost      = list_requests / 1000 × list_per_1000
retrieval_cost = retrieved_gib × retrieval_gib
egress_cost    = egress_gib × egress_gib_rate
object_total   = storage_cost + get_cost + put_cost + list_cost
                 + retrieval_cost + egress_cost + extra_monthly_cost
```

In the input rate card, `egress_gib_rate` above is named `rates.egress_gib`; the quantity is `object_access.egress_gib`. Likewise, retrieval price is `rates.retrieval_gib` and retrieved volume is `object_access.retrieved_gib`.

For the object-only candidate, `S = mutable_gib + immutable_gib`. Semantic eligibility is checked separately: the cost formula does not make every mutation contract compatible with object storage.

Include all billable operations in the supplied counts: range fan-out, metadata reads, retries, multipart activity, compaction, and listing as applicable. Place operations with equivalent prices into the corresponding bucket; if their prices differ, use an explicit additional-cost estimate. One logical application request need not equal one object request.

Retrieved bytes and egress bytes are separate quantities. A byte-range read need not retrieve the whole object, and retrieval does not necessarily create billable egress. The caller supplies actual billable assumptions for its deployment. [S3 pricing](https://aws.amazon.com/s3/pricing/) describes the categories that need review.

## Hybrid storage

```text
hybrid_total = block_total(mutable_gib + cache_gib, hybrid.block)
               + object_total(immutable_gib, hybrid.object_access)
```

The cache duplicates immutable data. **The object tier keeps the complete `immutable_gib` amount; cache capacity is never subtracted from it.** The block head/WAL and immutable tier are separate logical categories supplied by the caller.

Hybrid access estimates are explicit post-cache origin requests and bytes. No cache-hit ratio is inferred. Cache fills, misses, refreshes, and writes still have costs and must be included in those estimates. Cache storage is priced even if its assumed request reduction proves ineffective.

## Comparison limits

The result is a storage-component comparison, not total cost of ownership. Compute, licenses, application replication, snapshots and backups, KMS, CDN, network gateways, monitoring, engineering effort, and taxes are excluded unless included in a documented `extra_monthly_cost` estimate.

Object egress has a dedicated field; block egress does not. If external clients receive the same payload from either design, account for block-serving egress in `block.extra_monthly_cost` as well. Otherwise explicitly restrict the comparison to a deployment with no relevant billable egress. Do not charge network delivery to one candidate and silently assume it is free for another.

An EBS volume is replicated [within one Availability Zone](https://docs.aws.amazon.com/ebs/latest/userguide/EBSFeatures.html). S3 Standard stores data [across multiple Availability Zones](https://docs.aws.amazon.com/AmazonS3/latest/userguide/DataDurability.html). `copies: 3` multiplies the modeled block resource costs; it does not implement replication, failover, consistency, backups, or equivalent durability. Availability architecture must be assessed separately.

This model does not implement price tiers, volume discounts, free allowances, minimum billable object sizes, minimum storage durations, archive restore behavior, or account-specific contracts. Do not plug an archival capacity rate into the Standard-like model and assume a complete comparison.

Differences should be described as **modeled monthly cost differences under the supplied assumptions**. Real savings require an observed baseline, a deployed change, and comparable billing and workload measurements. Storepath supplies none of those by itself.

## Arithmetic and validation

Inputs must be finite and nonnegative; block copies must be at least one. Cost calculation uses unrounded values and rounds only for display. Increasing a billed quantity or nonnegative rate should not decrease its corresponding line item. A zero-cost baseline cannot support a meaningful percentage reduction.

Review a decision's assumptions before its total: changing access counts, retention, provisioning headroom, replication, or cache effectiveness can change which plan is least expensive.
