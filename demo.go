package storepath

import (
	"embed"
	"fmt"
)

//go:embed examples/*.json
var scenarios embed.FS

// Demo evaluates six embedded, synthetic customer scenarios and checks their
// semantic outcomes. It requires no files, credentials, network, or cloud spend.
func Demo() ([]Decision, error) {
	cases := []struct {
		name   string
		mode   Mode
		status string
	}{
		{"transactional-db", Block, "modeled"},
		{"telemetry-retention", Hybrid, "modeled"},
		{"backup-repository", Object, "modeled"},
		{"analytics-range-reads", Object, "modeled"},
		{"hot-object-service", Hybrid, "modeled"},
		{"strict-latency", "", "benchmark_required"},
	}
	results := make([]Decision, 0, len(cases))
	for _, c := range cases {
		f, err := scenarios.Open("examples/" + c.name + ".json")
		if err != nil {
			return nil, err
		}
		in, err := Decode(f)
		closeErr := f.Close()
		if err != nil {
			return nil, err
		}
		if closeErr != nil {
			return nil, closeErr
		}
		d, err := Evaluate(in)
		if err != nil {
			return nil, err
		}
		if d.Recommendation != c.mode || d.Status != c.status {
			return nil, fmt.Errorf("demo %s: got %s/%s, expected %s/%s", c.name, d.Recommendation, d.Status, c.mode, c.status)
		}
		results = append(results, d)
	}
	return results, nil
}
