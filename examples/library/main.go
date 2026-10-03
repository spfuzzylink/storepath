// This example imports the public library and inspects a planning decision.
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/spfuzzylink/storepath"
)

func main() {
	f, err := os.Open("examples/telemetry-retention.json")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	in, err := storepath.Decode(f)
	if err != nil {
		log.Fatal(err)
	}
	decision, err := storepath.Evaluate(in)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s: %s (%s)\n", decision.Workload, decision.Recommendation, decision.Status)
	for _, c := range decision.Candidates {
		if c.Cost != nil {
			fmt.Printf("%s: %s %.2f/month\n", c.Mode, c.Cost.Currency, c.Cost.MonthlyTotal)
		}
	}
}
