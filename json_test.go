package storepath_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sp "github.com/spfuzzylink/storepath"
)

func profileJSON(t testing.TB) string {
	t.Helper()
	b, err := json.Marshal(profile())
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestDecodeRejectsAmbiguousOrMalformedDocuments(t *testing.T) {
	valid := profileJSON(t)
	for _, tc := range []struct{ name, document, errorPart string }{
		{"unknown top field", strings.Replace(valid, `"schema_version":1`, `"schema_version":1,"schema_typo":1`, 1), "unknown field"},
		{"unknown nested field", strings.Replace(valid, `"copies":1`, `"copies":1,"copiez":2`, 1), "unknown field"},
		{"duplicate top key", strings.Replace(valid, `"schema_version":1`, `"schema_version":1,"schema_version":1`, 1), "duplicate"},
		{"duplicate nested key", strings.Replace(valid, `"copies":1`, `"copies":1,"copies":2`, 1), "duplicate"},
		{"folded duplicate key", strings.Replace(valid, `"copies":1`, `"copies":1,"COPIES":2`, 1), "duplicate"},
		{"escaped duplicate key", strings.Replace(valid, `"copies":1`, `"copies":1,"copi\u0065s":2`, 1), "duplicate"},
		{"missing rate field", strings.Replace(valid, `"get_per_1000":0.0004,`, "", 1), ""},
		{"missing semantic flag", strings.Replace(valid, `"requires_posix":false,`, "", 1), ""},
		{"trailing document", valid + valid, "exactly one"},
		{"trailing garbage", valid + "oops", "exactly one"},
		{"null document", "null", "null"},
		{"null optional field", strings.Replace(valid, `"name":`, `"hybrid":null,"name":`, 1), "null"},
		{"null scalar", strings.Replace(valid, `"copies":1`, `"copies":null`, 1), "null"},
		{"missing document", "", ""},
		{"array", "[]", ""},
		{"scalar", "17", ""},
		{"truncated object", valid[:len(valid)-1], ""},
		{"oversized float", strings.Replace(valid, `"immutable_gib":100`, `"immutable_gib":1e9999`, 1), ""},
		{"wrong numeric type", strings.Replace(valid, `"copies":1`, `"copies":"one"`, 1), ""},
		{"deep nesting", strings.Repeat("[", 34) + "0" + strings.Repeat("]", 34), "nesting"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := sp.Decode(strings.NewReader(tc.document))
			if err == nil {
				t.Fatal("invalid document accepted")
			}
			if tc.errorPart != "" && !strings.Contains(err.Error(), tc.errorPart) {
				t.Fatalf("want %q error, got %v", tc.errorPart, err)
			}
		})
	}
}

func TestDecodeInputSizeBoundary(t *testing.T) {
	valid := profileJSON(t)
	atLimit := valid + strings.Repeat(" ", sp.MaxInputBytes-len(valid))
	if _, err := sp.Decode(strings.NewReader(atLimit)); err != nil {
		t.Fatalf("exact size limit rejected: %v", err)
	}
	if _, err := sp.Decode(strings.NewReader(atLimit + " ")); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("want size-limit error, got %v", err)
	}
}

type failedReader struct{ err error }

func (r failedReader) Read([]byte) (int, error) { return 0, r.err }

func TestDecodePropagatesReaderFailure(t *testing.T) {
	want := errors.New("synthetic read failure")
	if _, err := sp.Decode(failedReader{want}); !errors.Is(err, want) {
		t.Fatalf("lost reader error: %v", err)
	}
}

func TestDecodeEveryBundledJSONExample(t *testing.T) {
	paths, err := filepath.Glob("examples/*.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 6 {
		t.Fatalf("want six fixture documents, got %d", len(paths))
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			in, err := sp.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			d, err := sp.Evaluate(in)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := json.Marshal(d); err != nil {
				t.Fatalf("result cannot be serialized: %v", err)
			}
		})
	}
}

func FuzzDecode(f *testing.F) {
	f.Add([]byte(profileJSON(f)))
	f.Add([]byte(`{"schema_version":1,"schema_version":2}`))
	f.Add([]byte("null"))
	f.Add([]byte(strings.Repeat("[", 34) + "0" + strings.Repeat("]", 34)))
	f.Fuzz(func(t *testing.T, data []byte) {
		// The boundary is exercised separately; keep mutation work bounded.
		if len(data) > 64*1024 {
			t.Skip()
		}
		in, err := sp.Decode(bytes.NewReader(data))
		if err != nil {
			return
		}
		if err := sp.Validate(in); err != nil {
			t.Fatalf("decoder returned invalid input: %v", err)
		}
		d, err := sp.Evaluate(in)
		if err != nil {
			return
		} // Finite individual values can overflow a billed total.
		if _, err := json.Marshal(d); err != nil {
			t.Fatalf("non-serializable decision: %v", err)
		}
	})
}
