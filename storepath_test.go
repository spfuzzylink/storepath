package storepath_test

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	sp "github.com/spfuzzylink/storepath"
)

func profile() sp.Input {
	return sp.Input{SchemaVersion: 1,
		Workload: sp.Workload{Name: "Synthetic test workload", ImmutableGiB: 100, Mutation: "immutable", Block: sp.BlockPlan{Copies: 1, ProvisionedIOPS: 3000, ProvisionedMiBps: 125}},
		Rates:    sp.Rates{Label: "Synthetic test rates", Currency: "USD", AsOf: "2026-10-03", Illustrative: true, BlockGiBMonth: .08, BlockIncludedIOPS: 3000, BlockIOPSMonth: .005, BlockIncludedMiBps: 125, BlockMiBpsMonth: .04, ObjectGiBMonth: .023, GetPer1000: .0004, PutPer1000: .005, ListPer1000: .005, EgressGiB: .09},
	}
}

func withHybrid() sp.Input {
	in := profile()
	in.Workload.CanSplitTiers = true
	in.Workload.Hybrid = &sp.HybridPlan{CacheGiB: 20, Block: sp.BlockPlan{Copies: 1, ProvisionedIOPS: 3000, ProvisionedMiBps: 125}}
	return in
}

func evaluate(t *testing.T, in sp.Input) sp.Decision {
	t.Helper()
	d, err := sp.Evaluate(in)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	return d
}

func candidate(t *testing.T, d sp.Decision, mode sp.Mode) sp.Candidate {
	t.Helper()
	for _, c := range d.Candidates {
		if c.Mode == mode {
			return c
		}
	}
	t.Fatalf("missing candidate %q", mode)
	return sp.Candidate{}
}

func item(t *testing.T, c sp.Candidate, name string) sp.LineItem {
	t.Helper()
	if c.Cost == nil {
		t.Fatalf("%s has no cost", c.Mode)
	}
	for _, x := range c.Cost.Items {
		if x.Name == name {
			return x
		}
	}
	t.Fatalf("%s missing item %q", c.Mode, name)
	return sp.LineItem{}
}

func closeTo(t *testing.T, got, want float64) {
	t.Helper()
	if math.IsNaN(got) || math.IsInf(got, 0) || math.Abs(got-want) > 1e-10*math.Max(1, math.Abs(want)) {
		t.Fatalf("got %.15g, want %.15g", got, want)
	}
}

func fixture(t *testing.T, name string) sp.Input {
	t.Helper()
	f, err := os.Open(filepath.Join("examples", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	in, err := sp.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	return in
}

func TestSemanticRequirementsCannotBeOverriddenByFreeObjectStorage(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*sp.Input)
	}{
		{"POSIX", func(in *sp.Input) { in.Workload.RequiresPOSIX = true }},
		{"fsync", func(in *sp.Input) { in.Workload.RequiresFsync = true }},
		{"random mutation", func(in *sp.Input) { in.Workload.Mutation = "random_in_place"; in.Workload.MutableGiB = 1 }},
		{"unsealed append", func(in *sp.Input) { in.Workload.Mutation = "append_and_seal"; in.Workload.MutableGiB = 1 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := profile()
			tc.change(&in)
			in.Rates.ObjectGiBMonth = 0
			d := evaluate(t, in)
			c := candidate(t, d, sp.Object)
			if c.Compatible || c.Cost != nil || len(c.Reasons) == 0 {
				t.Fatalf("object gate not explained: %+v", c)
			}
			if d.Recommendation != sp.Block {
				t.Fatalf("free incompatible storage won: %+v", d)
			}
		})
	}
}

func TestWholeObjectReplacementAndRangeReadsRemainEligible(t *testing.T) {
	in := profile()
	in.Workload.Mutation = "replace_object"
	in.Workload.MutableGiB = 10
	if !candidate(t, evaluate(t, in), sp.Object).Compatible {
		t.Fatal("whole-object replacement rejected")
	}
	in = fixture(t, "analytics-range-reads")
	d := evaluate(t, in)
	if !candidate(t, d, sp.Object).Compatible || d.Recommendation != sp.Object {
		t.Fatalf("range-read example: %+v", d)
	}
}

func TestHybridRequiresAnActualTierBoundary(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*sp.Input)
	}{
		{"cannot split", func(in *sp.Input) { in.Workload.CanSplitTiers = false }},
		{"missing plan", func(in *sp.Input) { in.Workload.Hybrid = nil }},
		{"no immutable tier", func(in *sp.Input) {
			in.Workload.ImmutableGiB = 0
			in.Workload.MutableGiB = 100
			in.Workload.Mutation = "replace_object"
			in.Workload.Hybrid.CacheGiB = 0
		}},
		{"no active tier", func(in *sp.Input) { in.Workload.Hybrid.CacheGiB = 0 }},
		{"read cache cannot provide POSIX", func(in *sp.Input) { in.Workload.RequiresPOSIX = true }},
		{"read cache cannot provide fsync", func(in *sp.Input) { in.Workload.RequiresFsync = true }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := withHybrid()
			tc.change(&in)
			c := candidate(t, evaluate(t, in), sp.Hybrid)
			if c.Compatible || c.Cost != nil || len(c.Reasons) == 0 {
				t.Fatalf("ineligible hybrid offered: %+v", c)
			}
		})
	}
}

func TestHybridDuplicatesCachedBytesAndUsesExplicitOriginOperations(t *testing.T) {
	in := withHybrid()
	in.Workload.MutableGiB = 10
	in.Workload.Mutation = "append_and_seal"
	in.Workload.ObjectAccess = sp.ObjectAccess{GetRequests: 900000, PutRequests: 80000, RetrievedGiB: 7000}
	in.Workload.Hybrid.ObjectAccess = sp.ObjectAccess{GetRequests: 1234, PutRequests: 567, ListRequests: 8, RetrievedGiB: 9, EgressGiB: 10}
	c := candidate(t, evaluate(t, in), sp.Hybrid)
	closeTo(t, item(t, c, "block_capacity").Quantity, 30)
	closeTo(t, item(t, c, "object_capacity").Quantity, 100)
	closeTo(t, item(t, c, "object_get_head").Quantity, 1.234)
	closeTo(t, item(t, c, "object_put_copy_post").Quantity, .567)
	closeTo(t, item(t, c, "object_list").Quantity, .008)
	closeTo(t, item(t, c, "object_retrieval").Quantity, 9)
	closeTo(t, item(t, c, "object_egress").Quantity, 10)
	in.Workload.Hybrid.CacheGiB = 40
	larger := candidate(t, evaluate(t, in), sp.Hybrid)
	closeTo(t, item(t, larger, "object_capacity").Quantity, 100)
	closeTo(t, item(t, larger, "object_get_head").Quantity, 1.234)
	if larger.Cost.MonthlyTotal <= c.Cost.MonthlyTotal {
		t.Fatal("larger cache silently invented access savings")
	}
}

func TestCopiesMultiplyProvisionedResourcesButNotExtraMonthlyTotal(t *testing.T) {
	in := profile()
	in.Workload.Block = sp.BlockPlan{Copies: 3, HeadroomGiB: 10, ProvisionedIOPS: 4000, ProvisionedMiBps: 200, ExtraMonthlyCost: 7}
	c := candidate(t, evaluate(t, in), sp.Block)
	closeTo(t, item(t, c, "block_capacity").Quantity, 330)
	closeTo(t, item(t, c, "block_extra_iops").Quantity, 3000)
	closeTo(t, item(t, c, "block_extra_throughput").Quantity, 225)
	closeTo(t, item(t, c, "block_extra_monthly").Cost, 7)
	closeTo(t, c.Cost.MonthlyTotal, 57.4)
	// Below-baseline provisioning cannot create a negative performance credit.
	in.Workload.Block.ProvisionedIOPS = 100
	in.Workload.Block.ProvisionedMiBps = 10
	c = candidate(t, evaluate(t, in), sp.Block)
	closeTo(t, item(t, c, "block_extra_iops").Cost, 0)
	closeTo(t, item(t, c, "block_extra_throughput").Cost, 0)
}

func TestHandCalculatedMonthlyBillRetainsSubcentPrecision(t *testing.T) {
	in := profile()
	in.Workload.Mutation = "replace_object"
	in.Workload.MutableGiB = 3.125
	in.Workload.ImmutableGiB = 10.5
	in.Workload.Block = sp.BlockPlan{Copies: 3, HeadroomGiB: .375, ProvisionedIOPS: 4000, ProvisionedMiBps: 200, ExtraMonthlyCost: 1.1111}
	in.Workload.ObjectAccess = sp.ObjectAccess{GetRequests: 1234, PutRequests: 345, ListRequests: 7, RetrievedGiB: 2.25, EgressGiB: 4.5, ExtraMonthlyCost: .2222}
	in.Rates.RetrievalGiB = .01
	d := evaluate(t, in)
	// Block: 42 GiB * .08 + 3000 IOPS * .005 + 225 MiB/s * .04 + 1.1111.
	closeTo(t, candidate(t, d, sp.Block).Cost.MonthlyTotal, 28.4711)
	// Object: .313375 capacity + .0004936 GET + .001725 PUT + .000035 LIST
	//         + .0225 retrieval + .405 egress + .2222 explicit additional cost.
	closeTo(t, candidate(t, d, sp.Object).Cost.MonthlyTotal, .9653286)
	if d.BlockDifference == nil {
		t.Fatal("missing cost difference")
	}
	closeTo(t, *d.BlockDifference, 27.5057714)
	for _, c := range d.Candidates {
		if c.Cost == nil {
			continue
		}
		var total float64
		for _, li := range c.Cost.Items {
			total += li.Cost
		}
		closeTo(t, total, c.Cost.MonthlyTotal)
	}
}

func TestLatencyEvidenceControlsSelection(t *testing.T) {
	for _, tc := range []struct {
		name           string
		measurements   map[sp.Mode]float64
		status         string
		recommendation sp.Mode
	}{
		{"exact target passes", map[sp.Mode]float64{sp.Object: 5}, "latency_evidence_satisfied", sp.Object},
		{"missing evidence", nil, "benchmark_required", ""},
		{"all fail", map[sp.Mode]float64{sp.Block: 6, sp.Object: 7}, "no_feasible_plan", ""},
		{"passing expensive beats unmeasured cheap", map[sp.Mode]float64{sp.Block: 4}, "latency_evidence_satisfied", sp.Block},
		{"passing expensive beats failing cheap", map[sp.Mode]float64{sp.Block: 4, sp.Object: 6}, "latency_evidence_satisfied", sp.Block},
		{"cheapest passing", map[sp.Mode]float64{sp.Block: 4, sp.Object: 3}, "latency_evidence_satisfied", sp.Object},
		{"failure with missing evidence", map[sp.Mode]float64{sp.Block: 6}, "benchmark_required", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := profile()
			in.Workload.TargetP99MS = 5
			in.Workload.MeasuredP99MS = tc.measurements
			d := evaluate(t, in)
			if d.Status != tc.status || d.Recommendation != tc.recommendation {
				t.Fatalf("got %s/%s, want %s/%s", d.Status, d.Recommendation, tc.status, tc.recommendation)
			}
			if tc.recommendation == "" && d.BlockDifference != nil {
				t.Fatal("no recommendation must not imply a cost saving")
			}
		})
	}
	// An incompatible object's passing measurement cannot satisfy the contract.
	in := profile()
	in.Workload.RequiresFsync = true
	in.Workload.TargetP99MS = 5
	in.Workload.MeasuredP99MS = map[sp.Mode]float64{sp.Block: 6, sp.Object: 1}
	if d := evaluate(t, in); d.Status != "no_feasible_plan" {
		t.Fatalf("incompatible measured plan won: %+v", d)
	}
}

func TestEvaluateDoesNotMutateInputAndIsDeterministic(t *testing.T) {
	in := withHybrid()
	in.Workload.TargetP99MS = 5
	in.Workload.MeasuredP99MS = map[sp.Mode]float64{sp.Block: 2, sp.Object: 4, sp.Hybrid: 3}
	before, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	one, two := evaluate(t, in), evaluate(t, in)
	after, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("Evaluate mutated the caller's profile")
	}
	if !reflect.DeepEqual(one, two) {
		t.Fatal("same profile produced different decisions")
	}
	// Result mutation must not change the input map either.
	for _, c := range one.Candidates {
		if c.Latency.MeasuredP99MS != nil {
			*c.Latency.MeasuredP99MS = 999
		}
	}
	if in.Workload.MeasuredP99MS[sp.Block] != 2 {
		t.Fatal("result latency aliases caller data")
	}
}

func TestCostsAreMonotoneInBilledQuantitiesAndRates(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mode   sp.Mode
		change func(*sp.Input)
	}{
		{"object capacity", sp.Object, func(in *sp.Input) { in.Workload.ImmutableGiB *= 2 }},
		{"object rate", sp.Object, func(in *sp.Input) { in.Rates.ObjectGiBMonth *= 2 }},
		{"GET volume", sp.Object, func(in *sp.Input) { in.Workload.ObjectAccess.GetRequests += 1000 }},
		{"GET price", sp.Object, func(in *sp.Input) { in.Rates.GetPer1000 *= 2 }},
		{"egress", sp.Object, func(in *sp.Input) { in.Workload.ObjectAccess.EgressGiB += 1 }},
		{"block copies", sp.Block, func(in *sp.Input) { in.Workload.Block.Copies++ }},
		{"block headroom", sp.Block, func(in *sp.Input) { in.Workload.Block.HeadroomGiB += 1 }},
		{"block price", sp.Block, func(in *sp.Input) { in.Rates.BlockGiBMonth *= 2 }},
		{"block IOPS", sp.Block, func(in *sp.Input) { in.Workload.Block.ProvisionedIOPS += 1000 }},
		{"block throughput", sp.Block, func(in *sp.Input) { in.Workload.Block.ProvisionedMiBps += 100 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := profile()
			in.Workload.ObjectAccess.GetRequests = 1000
			before := candidate(t, evaluate(t, in), tc.mode).Cost.MonthlyTotal
			tc.change(&in)
			after := candidate(t, evaluate(t, in), tc.mode).Cost.MonthlyTotal
			if after <= before {
				t.Fatalf("cost did not increase: %g -> %g", before, after)
			}
		})
	}
}

func TestZeroRatesProduceFiniteZeroDifference(t *testing.T) {
	in := profile()
	in.Rates = sp.Rates{Label: "All zero synthetic rates", Currency: "USD", AsOf: "2026-10-03", Illustrative: true}
	d := evaluate(t, in)
	if d.BlockDifference == nil {
		t.Fatal("missing zero difference")
	}
	closeTo(t, *d.BlockDifference, 0)
	if _, err := json.Marshal(d); err != nil {
		t.Fatalf("decision is not finite JSON: %v", err)
	}
}

func TestBundledScenarioOutcomes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mode   sp.Mode
		status string
	}{
		{"transactional-db", sp.Block, "modeled"}, {"telemetry-retention", sp.Hybrid, "modeled"},
		{"backup-repository", sp.Object, "modeled"}, {"analytics-range-reads", sp.Object, "modeled"},
		{"hot-object-service", sp.Hybrid, "modeled"}, {"strict-latency", "", "benchmark_required"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := evaluate(t, fixture(t, tc.name))
			if d.Recommendation != tc.mode || d.Status != tc.status {
				t.Fatalf("got %s/%s, want %s/%s", d.Recommendation, d.Status, tc.mode, tc.status)
			}
			if !d.Illustrative || len(d.Assumptions) == 0 || len(d.Exclusions) == 0 {
				t.Fatal("example lost its modeling context")
			}
		})
	}
}

func TestInvalidProfiles(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*sp.Input)
	}{
		{"schema", func(in *sp.Input) { in.SchemaVersion = 2 }},
		{"name", func(in *sp.Input) { in.Workload.Name = "  " }},
		{"rate label", func(in *sp.Input) { in.Rates.Label = "" }},
		{"currency", func(in *sp.Input) { in.Rates.Currency = "usd" }},
		{"currency length", func(in *sp.Input) { in.Rates.Currency = "US" }},
		{"date", func(in *sp.Input) { in.Rates.AsOf = "2026-02-30" }},
		{"negative capacity", func(in *sp.Input) { in.Workload.ImmutableGiB = -1 }},
		{"empty capacity", func(in *sp.Input) { in.Workload.ImmutableGiB = 0 }},
		{"NaN capacity", func(in *sp.Input) { in.Workload.ImmutableGiB = math.NaN() }},
		{"infinite rate", func(in *sp.Input) { in.Rates.ObjectGiBMonth = math.Inf(1) }},
		{"negative price", func(in *sp.Input) { in.Rates.GetPer1000 = -1 }},
		{"negative target", func(in *sp.Input) { in.Workload.TargetP99MS = -1 }},
		{"unknown mutation", func(in *sp.Input) { in.Workload.Mutation = "sometimes" }},
		{"immutable mutable data", func(in *sp.Input) { in.Workload.MutableGiB = 1 }},
		{"missing mutable working set", func(in *sp.Input) { in.Workload.Mutation = "random_in_place" }},
		{"zero copies", func(in *sp.Input) { in.Workload.Block.Copies = 0 }},
		{"negative headroom", func(in *sp.Input) { in.Workload.Block.HeadroomGiB = -1 }},
		{"infinite IOPS", func(in *sp.Input) { in.Workload.Block.ProvisionedIOPS = math.Inf(1) }},
		{"negative requests", func(in *sp.Input) { in.Workload.ObjectAccess.GetRequests = -1 }},
		{"NaN bytes", func(in *sp.Input) { in.Workload.ObjectAccess.EgressGiB = math.NaN() }},
		{"cache exceeds immutable data", func(in *sp.Input) { in.Workload.Hybrid.CacheGiB = 101 }},
		{"negative cache", func(in *sp.Input) { in.Workload.Hybrid.CacheGiB = -1 }},
		{"hybrid copies", func(in *sp.Input) { in.Workload.Hybrid.Block.Copies = 0 }},
		{"hybrid requests", func(in *sp.Input) { in.Workload.Hybrid.ObjectAccess.PutRequests = -1 }},
		{"unknown measurement mode", func(in *sp.Input) { in.Workload.MeasuredP99MS = map[sp.Mode]float64{"unknown": 5} }},
		{"zero measurement", func(in *sp.Input) { in.Workload.MeasuredP99MS = map[sp.Mode]float64{sp.Object: 0} }},
		{"NaN measurement", func(in *sp.Input) { in.Workload.MeasuredP99MS = map[sp.Mode]float64{sp.Object: math.NaN()} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := withHybrid()
			tc.change(&in)
			if err := sp.Validate(in); err == nil {
				t.Fatal("Validate accepted invalid profile")
			}
			if _, err := sp.Evaluate(in); err == nil {
				t.Fatal("Evaluate accepted invalid profile")
			}
		})
	}
}

func TestFiniteInputsCannotOverflowCostOutput(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*sp.Input)
	}{
		{"block capacity", func(in *sp.Input) { in.Rates.BlockGiBMonth = math.MaxFloat64 }},
		{"object capacity", func(in *sp.Input) { in.Rates.ObjectGiBMonth = math.MaxFloat64 }},
		{"summed object components", func(in *sp.Input) {
			in.Rates.ObjectGiBMonth = 0
			in.Workload.ObjectAccess.ExtraMonthlyCost = math.MaxFloat64
			in.Workload.ObjectAccess.EgressGiB = 2
			in.Rates.EgressGiB = math.MaxFloat64 / 2
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := profile()
			tc.change(&in)
			if err := sp.Validate(in); err != nil {
				t.Fatalf("individual inputs should be finite: %v", err)
			}
			if _, err := sp.Evaluate(in); err == nil || !strings.Contains(err.Error(), "overflow") {
				t.Fatalf("want overflow error, got %v", err)
			}
		})
	}
}
