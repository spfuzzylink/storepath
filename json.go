package storepath

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// MaxInputBytes bounds CLI/library JSON parsing. Profiles are small documents.
const MaxInputBytes = 2 << 20

// Decode rejects unknown keys, duplicate keys, trailing documents and null
// values. Omitting optional measurements is different from passing null.
func Decode(r io.Reader) (Input, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxInputBytes+1))
	if err != nil {
		return Input{}, err
	}
	if len(data) > MaxInputBytes {
		return Input{}, fmt.Errorf("input exceeds %d bytes", MaxInputBytes)
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	if err := uniqueValue(d, 0); err != nil {
		return Input{}, err
	}
	if _, err := d.Token(); err != io.EOF {
		return Input{}, fmt.Errorf("expected exactly one JSON object")
	}
	if err := requiredFields(data); err != nil {
		return Input{}, err
	}
	var in Input
	d = json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&in); err != nil {
		return Input{}, err
	}
	if err := Validate(in); err != nil {
		return Input{}, err
	}
	return in, nil
}

// Require explicit zero/false values in billing and semantic inputs so a missing
// price or filesystem flag cannot silently become free or object-compatible.
// Programmatic Go callers use normal zero-valued structs intentionally.
func requiredFields(data []byte) error {
	object := func(raw []byte, path string, fields ...string) (map[string]json.RawMessage, error) {
		var m map[string]json.RawMessage
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, fmt.Errorf("%s must be an object", path)
		}
		for _, field := range fields {
			if _, ok := m[field]; !ok {
				return nil, fmt.Errorf("%s.%s must be explicit (zero/false is allowed)", path, field)
			}
		}
		return m, nil
	}
	top, err := object(data, "input", "schema_version", "workload", "rates")
	if err != nil {
		return err
	}
	w, err := object(top["workload"], "workload", "name", "mutable_gib", "immutable_gib", "mutation", "requires_posix", "requires_fsync", "can_split_tiers", "block", "object_access")
	if err != nil {
		return err
	}
	_, err = object(top["rates"], "rates", "label", "currency", "as_of", "illustrative", "block_gib_month", "block_included_iops", "block_iops_month", "block_included_mibps", "block_mibps_month", "object_gib_month", "get_per_1000", "put_per_1000", "list_per_1000", "retrieval_gib", "egress_gib")
	if err != nil {
		return err
	}
	blockFields := []string{"copies", "headroom_gib", "provisioned_iops", "provisioned_mibps", "extra_monthly_cost"}
	accessFields := []string{"get_requests", "put_requests", "list_requests", "retrieved_gib", "egress_gib", "extra_monthly_cost"}
	if _, err = object(w["block"], "workload.block", blockFields...); err != nil {
		return err
	}
	if _, err = object(w["object_access"], "workload.object_access", accessFields...); err != nil {
		return err
	}
	if raw, ok := w["hybrid"]; ok {
		h, err := object(raw, "workload.hybrid", "cache_gib", "block", "object_access")
		if err != nil {
			return err
		}
		if _, err = object(h["block"], "workload.hybrid.block", blockFields...); err != nil {
			return err
		}
		if _, err = object(h["object_access"], "workload.hybrid.object_access", accessFields...); err != nil {
			return err
		}
	}
	return nil
}

func uniqueValue(d *json.Decoder, depth int) error {
	if depth > 32 {
		return fmt.Errorf("JSON nesting exceeds 32 levels")
	}
	t, err := d.Token()
	if err != nil {
		return err
	}
	if t == nil {
		return fmt.Errorf("null values are not allowed; omit optional fields instead")
	}
	delim, ok := t.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		keys := map[string]bool{}
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return err
			}
			s, ok := key.(string)
			if !ok {
				return fmt.Errorf("object key is not a string")
			}
			canonical := strings.ToLower(s)
			if keys[canonical] {
				return fmt.Errorf("duplicate JSON key %q", s)
			}
			keys[canonical] = true
			if err := uniqueValue(d, depth+1); err != nil {
				return err
			}
		}
	case '[':
		for d.More() {
			if err := uniqueValue(d, depth+1); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unexpected JSON delimiter %q", delim)
	}
	_, err = d.Token()
	return err
}
