//go:build js && wasm

// The browser adapter deliberately uses the same decoder and evaluator as the CLI.
package main

import (
	"encoding/json"
	"strings"
	"syscall/js"

	"github.com/spfuzzylink/storepath"
)

type response struct {
	Decision *storepath.Decision `json:"decision,omitempty"`
	Input    *storepath.Input    `json:"input,omitempty"`
	Error    string              `json:"error,omitempty"`
}

func encode(r response) string {
	b, err := json.Marshal(r)
	if err != nil {
		return `{"error":"The result could not be encoded."}`
	}
	return string(b)
}

func evaluate(_ js.Value, args []js.Value) (result any) {
	defer func() {
		if recover() != nil {
			result = encode(response{Error: "The evaluation failed. Check the input and reload the demo."})
		}
	}()
	if len(args) != 1 || args[0].Type() != js.TypeString {
		return encode(response{Error: "Provide one JSON input string."})
	}
	in, err := storepath.Decode(strings.NewReader(args[0].String()))
	if err != nil {
		return encode(response{Error: err.Error()})
	}
	d, err := storepath.Evaluate(in)
	if err != nil {
		return encode(response{Error: err.Error()})
	}
	// Return the canonical decoded schema for UI controls. The Go decoder can
	// accept differently cased optional keys; raw JS objects must not hide them.
	return encode(response{Decision: &d, Input: &in})
}

func main() {
	callback := js.FuncOf(evaluate)
	js.Global().Set("storepathEvaluate", callback)
	select {}
}
