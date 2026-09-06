package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
)

func decodeJSON(raw json.RawMessage) (any, error) {
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, fmt.Errorf("%w: empty or null JSON", ErrMalformedAuthority)
	}
	if !json.Valid(raw) {
		return nil, fmt.Errorf("%w: invalid JSON", ErrMalformedAuthority)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformedAuthority, err)
	}
	if dec.More() {
		return nil, fmt.Errorf("%w: trailing data", ErrMalformedAuthority)
	}
	return v, nil
}

func requireJSONObject(field string, raw json.RawMessage) (map[string]any, error) {
	v, err := decodeJSON(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, field)
	}
	obj, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%w: %s must be an object", ErrMalformedAuthority, field)
	}
	return obj, nil
}

func requireJSONArray(field string, raw json.RawMessage) ([]any, error) {
	v, err := decodeJSON(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, field)
	}
	arr, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("%w: %s must be an array", ErrMalformedAuthority, field)
	}
	return arr, nil
}

func canonicalValue(v any) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return "", fmt.Errorf("%w: %v", ErrMalformedAuthority, err)
	}
	return strings.TrimSpace(buf.String()), nil
}

func valuesEqual(a, b any) (bool, error) {
	ca, err := canonicalValue(a)
	if err != nil {
		return false, err
	}
	cb, err := canonicalValue(b)
	if err != nil {
		return false, err
	}
	return ca == cb, nil
}

func parseDecimal(v any) (*big.Rat, error) {
	switch n := v.(type) {
	case json.Number:
		r, ok := new(big.Rat).SetString(n.String())
		if !ok {
			return nil, fmt.Errorf("%w: not a decimal", ErrMalformedAuthority)
		}
		return r, nil
	case string:
		trimmed := strings.TrimSpace(n)
		if trimmed == "" {
			return nil, fmt.Errorf("%w: empty decimal", ErrMalformedAuthority)
		}
		r, ok := new(big.Rat).SetString(trimmed)
		if !ok {
			return nil, fmt.Errorf("%w: not a decimal", ErrMalformedAuthority)
		}
		return r, nil
	default:
		return nil, fmt.Errorf("%w: decimal must be a number or numeric string", ErrMalformedAuthority)
	}
}

func asString(v any) (string, bool) {
	s, ok := v.(string)
	return s, ok
}

func asStringSlice(v any) ([]string, bool) {
	arr, ok := v.([]any)
	if !ok {
		return nil, false
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		s, ok := item.(string)
		if !ok {
			return nil, false
		}
		out = append(out, s)
	}
	return out, true
}

func stringSet(items []string) map[string]struct{} {
	out := make(map[string]struct{}, len(items))
	for _, item := range items {
		out[item] = struct{}{}
	}
	return out
}

func forbiddenWildcard(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return true
	}
	if trimmed == "*" || trimmed == "**" || strings.EqualFold(trimmed, "any") {
		return true
	}
	return strings.Contains(trimmed, "*")
}
