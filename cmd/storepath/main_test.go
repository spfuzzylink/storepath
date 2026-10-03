package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/spfuzzylink/storepath"
)

func TestDemoTextAndJSON(t *testing.T) {
	for _, args := range [][]string{{"demo"}, {"demo", "--json"}} {
		var out, errout bytes.Buffer
		if code := run(args, strings.NewReader(""), &out, &errout); code != 0 {
			t.Fatalf("code=%d: %s", code, errout.String())
		}
		if len(args) == 1 {
			if strings.Count(out.String(), "PASS  ") != 6 {
				t.Fatal(out.String())
			}
			if !strings.Contains(out.String(), "No storage was provisioned or benchmarked") {
				t.Fatal("missing model scope")
			}
		} else {
			var ds []storepath.Decision
			if err := json.Unmarshal(out.Bytes(), &ds); err != nil {
				t.Fatal(err)
			}
			if len(ds) != 6 || ds[5].Status != "benchmark_required" {
				t.Fatalf("unexpected results: %+v", ds)
			}
		}
	}
}

func TestEvaluateFromStdin(t *testing.T) {
	data, err := os.ReadFile("../../examples/strict-latency.json")
	if err != nil {
		t.Fatal(err)
	}
	var out, errout bytes.Buffer
	if code := run([]string{"evaluate", "-"}, bytes.NewReader(data), &out, &errout); code != 0 {
		t.Fatalf("code=%d %s", code, errout.String())
	}
	var d storepath.Decision
	if err := json.Unmarshal(out.Bytes(), &d); err != nil {
		t.Fatal(err)
	}
	if d.Status != "benchmark_required" || d.Recommendation != "" {
		t.Fatalf("unmeasured latency got recommendation: %+v", d)
	}
}

func TestEvaluateFile(t *testing.T) {
	var out, errout bytes.Buffer
	if code := run([]string{"evaluate", "../../examples/transactional-db.json"}, strings.NewReader(""), &out, &errout); code != 0 {
		t.Fatalf("code=%d %s", code, errout.String())
	}
	var d storepath.Decision
	if err := json.Unmarshal(out.Bytes(), &d); err != nil {
		t.Fatal(err)
	}
	if d.Recommendation != storepath.Block {
		t.Fatalf("got %s", d.Recommendation)
	}
}

func TestInvalidUsageAndInput(t *testing.T) {
	for _, args := range [][]string{{"unknown"}, {"evaluate"}, {"evaluate", "-", "extra"}, {"evaluate", "missing-profile.json"}, {"demo", "--bogus"}, {"version", "extra"}, {"evaluate", "-"}} {
		var out, errout bytes.Buffer
		if code := run(args, strings.NewReader(`{"schema_version":1,"workload":null}`), &out, &errout); code != 2 {
			t.Fatalf("args=%v code=%d", args, code)
		}
		if out.Len() != 0 || errout.Len() == 0 {
			t.Fatalf("args=%v stdout=%q stderr=%q", args, out.String(), errout.String())
		}
	}
}

func TestVersionAndHelp(t *testing.T) {
	for _, args := range [][]string{nil, {"help"}, {"--help"}, {"version"}} {
		var out, errout bytes.Buffer
		if code := run(args, strings.NewReader(""), &out, &errout); code != 0 || out.Len() == 0 || errout.Len() != 0 {
			t.Fatalf("args=%v code=%d", args, code)
		}
	}
}
