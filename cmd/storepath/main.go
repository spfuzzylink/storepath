package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spfuzzylink/storepath"
)

var version = "dev"

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

func run(args []string, stdin io.Reader, out, errout io.Writer) int {
	fail := func(err error) int { fmt.Fprintln(errout, "storepath:", err); return 2 }
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(out, `Storepath - explain block, object, and hybrid storage choices

Usage:
  storepath demo [--json]    Run six synthetic scenarios; no cloud account needed
  storepath evaluate FILE   Evaluate a versioned JSON profile (use - for stdin)
  storepath version         Print version
  storepath help            Show this help

Valid evaluations exit 0, including benchmark_required/no_feasible_plan.
Invalid inputs or usage exit 2. Read the JSON status before using a recommendation.
Costs are partial estimates from supplied rates, not measured savings or TCO.
`)
		return 0
	}
	switch args[0] {
	case "version":
		if len(args) != 1 {
			return fail(fmt.Errorf("version accepts no arguments"))
		}
		fmt.Fprintln(out, version)
		return 0
	case "evaluate":
		if len(args) != 2 {
			return fail(fmt.Errorf("evaluate requires one JSON filename or -"))
		}
		var r io.Reader = stdin
		if args[1] != "-" {
			f, err := os.Open(args[1])
			if err != nil {
				return fail(err)
			}
			defer f.Close()
			r = f
		}
		in, err := storepath.Decode(r)
		if err != nil {
			return fail(err)
		}
		d, err := storepath.Evaluate(in)
		if err != nil {
			return fail(err)
		}
		if err := writeJSON(out, d); err != nil {
			return fail(err)
		}
		return 0
	case "demo":
		asJSON := len(args) == 2 && args[1] == "--json"
		if len(args) > 2 || len(args) == 2 && !asJSON {
			return fail(fmt.Errorf("demo accepts only --json"))
		}
		results, err := storepath.Demo()
		if err != nil {
			fmt.Fprintln(errout, "demo failed:", err)
			return 1
		}
		if asJSON {
			if err := writeJSON(out, results); err != nil {
				return fail(err)
			}
			return 0
		}
		fmt.Fprintln(out, "STOREPATH DEMO | Synthetic customer profiles; illustrative monthly USD costs")
		for _, d := range results {
			choice := string(d.Recommendation)
			if choice == "" {
				choice = d.Status
			}
			fmt.Fprintf(out, "\nPASS  %s -> %s\n", d.Workload, choice)
			for _, c := range d.Candidates {
				if !c.Compatible {
					fmt.Fprintf(out, "      %-7s incompatible: %s\n", c.Mode, c.Reasons[0])
					continue
				}
				fmt.Fprintf(out, "      %-7s %s %9.2f/month  latency: %s\n", c.Mode, c.Cost.Currency, c.Cost.MonthlyTotal, c.Latency.Status)
			}
		}
		fmt.Fprintln(out, "\nSix scenario checks passed. No storage was provisioned or benchmarked.")
		fmt.Fprintln(out, "Block and object durability/availability are not assumed equivalent.")
		fmt.Fprintln(out, "Compute, operations, backups and network charges need explicit accounting.")
		fmt.Fprintln(out, "Use demo --json for line items, reasons, assumptions, and exclusions.")
		return 0
	default:
		return fail(fmt.Errorf("unknown command %q; use help", args[0]))
	}
}
func writeJSON(w io.Writer, v any) error {
	e := json.NewEncoder(w)
	e.SetIndent("", "  ")
	return e.Encode(v)
}
