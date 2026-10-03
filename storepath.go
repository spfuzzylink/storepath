// Package storepath evaluates block, direct object API, and hybrid storage plans.
// It is a deterministic planning library, not a storage driver or cloud client.
package storepath

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// Mode identifies a placement plan.
type Mode string

const (
	Block  Mode = "block"
	Object Mode = "object"
	Hybrid Mode = "hybrid"
)

// Input is a versioned evaluation request. Rate units are normalized to GiB.
type Input struct {
	SchemaVersion int      `json:"schema_version"`
	Workload      Workload `json:"workload"`
	Rates         Rates    `json:"rates"`
}

// Workload describes a conventional application's storage contract. MutableGiB
// and ImmutableGiB are disjoint logical footprints, not traffic volumes.
type Workload struct {
	Name          string           `json:"name"`
	MutableGiB    float64          `json:"mutable_gib"`
	ImmutableGiB  float64          `json:"immutable_gib"`
	Mutation      string           `json:"mutation"`
	RequiresPOSIX bool             `json:"requires_posix"`
	RequiresFsync bool             `json:"requires_fsync"`
	CanSplitTiers bool             `json:"can_split_tiers"`
	Block         BlockPlan        `json:"block"`
	ObjectAccess  ObjectAccess     `json:"object_access"`
	Hybrid        *HybridPlan      `json:"hybrid,omitempty"`
	TargetP99MS   float64          `json:"target_p99_ms,omitempty"`
	MeasuredP99MS map[Mode]float64 `json:"measured_p99_ms,omitempty"`
}

// BlockPlan describes identically sized volumes. Performance and headroom apply
// per volume. Copies affect billed resources, not durability guarantees.
type BlockPlan struct {
	Copies           int     `json:"copies"`
	HeadroomGiB      float64 `json:"headroom_gib"`
	ProvisionedIOPS  float64 `json:"provisioned_iops"`
	ProvisionedMiBps float64 `json:"provisioned_mibps"`
	ExtraMonthlyCost float64 `json:"extra_monthly_cost"`
}

// ObjectAccess counts billed API operations, including retries and range-read
// fan-out. It does not derive object operations from block IOPS.
type ObjectAccess struct {
	GetRequests      float64 `json:"get_requests"`
	PutRequests      float64 `json:"put_requests"`
	ListRequests     float64 `json:"list_requests"`
	RetrievedGiB     float64 `json:"retrieved_gib"`
	EgressGiB        float64 `json:"egress_gib"`
	ExtraMonthlyCost float64 `json:"extra_monthly_cost"`
}

// HybridPlan explicitly supplies origin access after caching/tier separation.
// CacheGiB duplicates immutable data; it never reduces object storage capacity.
type HybridPlan struct {
	CacheGiB     float64      `json:"cache_gib"`
	Block        BlockPlan    `json:"block"`
	ObjectAccess ObjectAccess `json:"object_access"`
}

// Rates is a caller-supplied normalized monthly rate card. Bundled examples are
// illustrative, not live cloud quotations. Object scope is Standard-like direct
// APIs, without archival minimum duration/size charges or managed file layers.
type Rates struct {
	Label              string  `json:"label"`
	Currency           string  `json:"currency"`
	AsOf               string  `json:"as_of"`
	Illustrative       bool    `json:"illustrative"`
	BlockGiBMonth      float64 `json:"block_gib_month"`
	BlockIncludedIOPS  float64 `json:"block_included_iops"`
	BlockIOPSMonth     float64 `json:"block_iops_month"`
	BlockIncludedMiBps float64 `json:"block_included_mibps"`
	BlockMiBpsMonth    float64 `json:"block_mibps_month"`
	ObjectGiBMonth     float64 `json:"object_gib_month"`
	GetPer1000         float64 `json:"get_per_1000"`
	PutPer1000         float64 `json:"put_per_1000"`
	ListPer1000        float64 `json:"list_per_1000"`
	RetrievalGiB       float64 `json:"retrieval_gib"`
	EgressGiB          float64 `json:"egress_gib"`
}

// LineItem preserves the inputs and unrounded cost of a billing component.
type LineItem struct {
	Name     string  `json:"name"`
	Quantity float64 `json:"quantity"`
	Unit     string  `json:"unit"`
	Rate     float64 `json:"rate"`
	Cost     float64 `json:"cost"`
}

// Cost is a partial monthly storage estimate, not total cost of ownership.
type Cost struct {
	Currency     string     `json:"currency"`
	MonthlyTotal float64    `json:"monthly_total"`
	Items        []LineItem `json:"items"`
}

type Latency struct {
	Status        string   `json:"status"`
	TargetP99MS   float64  `json:"target_p99_ms,omitempty"`
	MeasuredP99MS *float64 `json:"measured_p99_ms,omitempty"`
}

type Candidate struct {
	Mode       Mode     `json:"mode"`
	Compatible bool     `json:"compatible"`
	Reasons    []string `json:"reasons"`
	Cost       *Cost    `json:"cost,omitempty"`
	Latency    Latency  `json:"latency"`
}

// Decision distinguishes cost-only modeling from supplied latency evidence.
// BlockDifference is block cost minus recommendation cost; it is not realized
// savings or an availability-equivalent comparison.
type Decision struct {
	SchemaVersion   int         `json:"schema_version"`
	Workload        string      `json:"workload"`
	Status          string      `json:"status"`
	Recommendation  Mode        `json:"recommendation,omitempty"`
	Reason          string      `json:"reason"`
	Candidates      []Candidate `json:"candidates"`
	RateLabel       string      `json:"rate_label"`
	RateAsOf        string      `json:"rate_as_of"`
	Illustrative    bool        `json:"illustrative"`
	BlockDifference *float64    `json:"modeled_monthly_difference_from_block,omitempty"`
	Assumptions     []string    `json:"assumptions"`
	Exclusions      []string    `json:"exclusions"`
}

// Evaluate rejects invalid inputs, rules out incompatible application contracts,
// and selects the least expensive candidate that satisfies supplied evidence.
// With a latency target, missing measurements never count as a pass.
func Evaluate(in Input) (Decision, error) {
	if err := Validate(in); err != nil {
		return Decision{}, err
	}
	w, r := in.Workload, in.Rates
	d := Decision{SchemaVersion: 1, Workload: w.Name, RateLabel: r.Label, RateAsOf: r.AsOf, Illustrative: r.Illustrative,
		Candidates: make([]Candidate, 0, 3),
		Assumptions: []string{
			"Quantities and prices are caller-supplied monthly estimates normalized to GiB; no provider API was queried.",
			"Block and object plans are not assumed to have equivalent availability, durability, backup, or failover behavior.",
			"Hybrid origin request/byte counts are explicit assumptions after caching; cache size does not imply a hit ratio.",
			"Latency evidence must be representative end-to-end measurements supplied by the caller; the library does not benchmark storage.",
		}, Exclusions: []string{
			"Compute, cache servers, licenses, engineering effort, CDN, KMS, observability, backups, and block network transfer unless included in extra_monthly_cost.",
			"Archival storage classes, minimum billable object sizes/durations, managed file layers, S3 Express, discounts, taxes, and migration costs.",
		}}
	for _, mode := range []Mode{Block, Object, Hybrid} {
		c := Candidate{Mode: mode, Compatible: true, Reasons: []string{}, Latency: Latency{Status: "not_requested"}}
		switch mode {
		case Block:
			c.Reasons = append(c.Reasons, "A block volume with an appropriate filesystem can support this application contract; application-level durability still requires design and testing.")
			c.Cost = blockCost(w.MutableGiB+w.ImmutableGiB, w.Block, r)
		case Object:
			if w.RequiresPOSIX {
				c.Reasons = append(c.Reasons, "The active application requires POSIX semantics; a direct object API is not that filesystem contract.")
			}
			if w.RequiresFsync {
				c.Reasons = append(c.Reasons, "The active application requires fsync-style persistence; it needs redesign before using only direct object APIs.")
			}
			if w.Mutation == "random_in_place" || w.Mutation == "append_and_seal" {
				c.Reasons = append(c.Reasons, "The mutable path needs in-place or append-before-seal operations; direct Standard-like object APIs are not a drop-in replacement.")
			}
			c.Compatible = len(c.Reasons) == 0
			if c.Compatible {
				c.Reasons = append(c.Reasons, "Immutable or whole-object replacement fits a direct object API; byte-range reads remain compatible.")
				c.Cost = objectCost(w.MutableGiB+w.ImmutableGiB, w.ObjectAccess, r)
			}
		case Hybrid:
			if !w.CanSplitTiers {
				c.Reasons = append(c.Reasons, "The application has not declared that it can separate immutable data from its active path.")
			}
			if w.ImmutableGiB == 0 {
				c.Reasons = append(c.Reasons, "No immutable retained data is available for an object tier.")
			}
			if w.Hybrid == nil {
				c.Reasons = append(c.Reasons, "An explicit hybrid block/cache plan and origin access budget are required.")
			} else if w.MutableGiB+w.Hybrid.CacheGiB == 0 {
				c.Reasons = append(c.Reasons, "The hybrid plan has no mutable or cache working set on block storage.")
			}
			// A cache alone cannot make required filesystem or fsync operations on
			// immutable retained objects behave like a full filesystem.
			if w.Hybrid != nil && w.MutableGiB == 0 && (w.RequiresPOSIX || w.RequiresFsync || w.Mutation == "random_in_place" || w.Mutation == "append_and_seal") {
				c.Reasons = append(c.Reasons, "Required mutable/filesystem operations need an explicitly sized mutable block working set, not only an immutable read cache.")
			}
			c.Compatible = len(c.Reasons) == 0
			if c.Compatible {
				c.Reasons = append(c.Reasons, "Place the active working set/cache on block and the complete immutable retained set on object storage.")
				c.Cost = blockCost(w.MutableGiB+w.Hybrid.CacheGiB, w.Hybrid.Block, r)
				o := objectCost(w.ImmutableGiB, w.Hybrid.ObjectAccess, r)
				c.Cost.Items = append(c.Cost.Items, o.Items...)
				c.Cost.MonthlyTotal += o.MonthlyTotal
			}
		}
		if c.Cost != nil && !finite(c.Cost.MonthlyTotal) {
			return Decision{}, fmt.Errorf("%s cost overflow: reduce quantities or rates", mode)
		}
		if w.TargetP99MS > 0 {
			c.Latency = Latency{Status: "unmeasured", TargetP99MS: w.TargetP99MS}
			if ms, ok := w.MeasuredP99MS[mode]; ok {
				c.Latency.MeasuredP99MS = &ms
				c.Latency.Status = "passes"
				if ms > w.TargetP99MS {
					c.Latency.Status = "fails"
				}
			}
		}
		d.Candidates = append(d.Candidates, c)
	}
	best := -1
	unmeasured := false
	for i, c := range d.Candidates {
		if !c.Compatible {
			continue
		}
		if c.Latency.Status == "unmeasured" {
			unmeasured = true
			continue
		}
		if c.Latency.Status == "fails" {
			continue
		}
		if best < 0 || c.Cost.MonthlyTotal < d.Candidates[best].Cost.MonthlyTotal {
			best = i
		}
	}
	if best < 0 {
		d.Status = "no_feasible_plan"
		d.Reason = "All compatible candidates fail the supplied latency target. Revise the design or measure another configuration."
		if unmeasured {
			d.Status = "benchmark_required"
			d.Reason = "A latency target was supplied but no compatible candidate has passing measurements. Benchmark the unmeasured candidates before choosing."
		}
		return d, nil
	}
	d.Recommendation = d.Candidates[best].Mode
	d.Status = "modeled"
	d.Reason = "Lowest modeled monthly storage cost among semantically compatible candidates; no latency target was requested. Validate operational requirements before adopting."
	if w.TargetP99MS > 0 {
		d.Status = "latency_evidence_satisfied"
		d.Reason = "Lowest modeled cost among compatible candidates whose supplied p99 measurements meet the target. Unmeasured candidates were not treated as passing."
	}
	diff := d.Candidates[0].Cost.MonthlyTotal - d.Candidates[best].Cost.MonthlyTotal
	d.BlockDifference = &diff
	return d, nil
}

func blockCost(logical float64, b BlockPlan, r Rates) *Cost {
	c := &Cost{Currency: r.Currency, Items: []LineItem{}}
	add(c, "block_capacity", (logical+b.HeadroomGiB)*float64(b.Copies), "GiB-month", r.BlockGiBMonth)
	add(c, "block_extra_iops", math.Max(0, b.ProvisionedIOPS-r.BlockIncludedIOPS)*float64(b.Copies), "IOPS-month", r.BlockIOPSMonth)
	add(c, "block_extra_throughput", math.Max(0, b.ProvisionedMiBps-r.BlockIncludedMiBps)*float64(b.Copies), "MiB/s-month", r.BlockMiBpsMonth)
	add(c, "block_extra_monthly", 1, "month", b.ExtraMonthlyCost)
	return c
}
func objectCost(gib float64, a ObjectAccess, r Rates) *Cost {
	c := &Cost{Currency: r.Currency, Items: []LineItem{}}
	add(c, "object_capacity", gib, "GiB-month", r.ObjectGiBMonth)
	add(c, "object_get_head", a.GetRequests/1000, "1000 requests", r.GetPer1000)
	add(c, "object_put_copy_post", a.PutRequests/1000, "1000 requests", r.PutPer1000)
	add(c, "object_list", a.ListRequests/1000, "1000 requests", r.ListPer1000)
	add(c, "object_retrieval", a.RetrievedGiB, "GiB", r.RetrievalGiB)
	add(c, "object_egress", a.EgressGiB, "GiB", r.EgressGiB)
	add(c, "object_extra_monthly", 1, "month", a.ExtraMonthlyCost)
	return c
}
func add(c *Cost, name string, q float64, unit string, rate float64) {
	x := LineItem{Name: name, Quantity: q, Unit: unit, Rate: rate, Cost: q * rate}
	c.Items = append(c.Items, x)
	c.MonthlyTotal += x.Cost
}
func finite(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }

// Validate prevents missing contract details, invalid rate metadata, negative
// quantities, non-finite calculations, and impossible cache/lifecycle inputs.
func Validate(in Input) error {
	if in.SchemaVersion != 1 {
		return fmt.Errorf("schema_version must be 1")
	}
	w, r := in.Workload, in.Rates
	if strings.TrimSpace(w.Name) == "" {
		return fmt.Errorf("workload.name is required")
	}
	if strings.TrimSpace(r.Label) == "" {
		return fmt.Errorf("rates.label is required")
	}
	if len(r.Currency) != 3 || strings.Trim(r.Currency, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") != "" {
		return fmt.Errorf("rates.currency must be a three-letter uppercase code")
	}
	if _, err := time.Parse("2006-01-02", r.AsOf); err != nil {
		return fmt.Errorf("rates.as_of must be YYYY-MM-DD")
	}
	values := []struct {
		name  string
		value float64
	}{
		{"mutable_gib", w.MutableGiB}, {"immutable_gib", w.ImmutableGiB}, {"target_p99_ms", w.TargetP99MS},
		{"block_gib_month", r.BlockGiBMonth}, {"block_included_iops", r.BlockIncludedIOPS}, {"block_iops_month", r.BlockIOPSMonth},
		{"block_included_mibps", r.BlockIncludedMiBps}, {"block_mibps_month", r.BlockMiBpsMonth}, {"object_gib_month", r.ObjectGiBMonth},
		{"get_per_1000", r.GetPer1000}, {"put_per_1000", r.PutPer1000}, {"list_per_1000", r.ListPer1000}, {"retrieval_gib", r.RetrievalGiB}, {"egress_gib", r.EgressGiB},
	}
	for _, v := range values {
		if !finite(v.value) || v.value < 0 {
			return fmt.Errorf("%s must be finite and nonnegative", v.name)
		}
	}
	if w.MutableGiB+w.ImmutableGiB <= 0 || !finite(w.MutableGiB+w.ImmutableGiB) {
		return fmt.Errorf("total data footprint must be positive and finite")
	}
	switch w.Mutation {
	case "immutable":
		if w.MutableGiB != 0 {
			return fmt.Errorf("immutable mutation requires mutable_gib = 0")
		}
	case "replace_object", "random_in_place", "append_and_seal":
		if w.MutableGiB == 0 {
			return fmt.Errorf("%s requires a positive mutable_gib working set", w.Mutation)
		}
	default:
		return fmt.Errorf("mutation must be immutable, replace_object, random_in_place, or append_and_seal")
	}
	if err := validateBlock(w.Block); err != nil {
		return fmt.Errorf("block: %w", err)
	}
	if err := validateAccess(w.ObjectAccess); err != nil {
		return fmt.Errorf("object_access: %w", err)
	}
	if w.Hybrid != nil {
		if !finite(w.Hybrid.CacheGiB) || w.Hybrid.CacheGiB < 0 || w.Hybrid.CacheGiB > w.ImmutableGiB {
			return fmt.Errorf("hybrid.cache_gib must be finite and between zero and immutable_gib")
		}
		if err := validateBlock(w.Hybrid.Block); err != nil {
			return fmt.Errorf("hybrid.block: %w", err)
		}
		if err := validateAccess(w.Hybrid.ObjectAccess); err != nil {
			return fmt.Errorf("hybrid.object_access: %w", err)
		}
	}
	for mode, ms := range w.MeasuredP99MS {
		if mode != Block && mode != Object && mode != Hybrid {
			return fmt.Errorf("unknown latency mode %q", mode)
		}
		if !finite(ms) || ms <= 0 {
			return fmt.Errorf("measured_p99_ms.%s must be positive and finite", mode)
		}
	}
	return nil
}
func validateBlock(b BlockPlan) error {
	if b.Copies < 1 {
		return fmt.Errorf("copies must be at least 1")
	}
	for _, v := range []float64{b.HeadroomGiB, b.ProvisionedIOPS, b.ProvisionedMiBps, b.ExtraMonthlyCost} {
		if !finite(v) || v < 0 {
			return fmt.Errorf("quantities must be finite and nonnegative")
		}
	}
	return nil
}
func validateAccess(a ObjectAccess) error {
	for _, v := range []float64{a.GetRequests, a.PutRequests, a.ListRequests, a.RetrievedGiB, a.EgressGiB, a.ExtraMonthlyCost} {
		if !finite(v) || v < 0 {
			return fmt.Errorf("quantities must be finite and nonnegative")
		}
	}
	return nil
}
