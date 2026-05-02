package handlers

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// CheckParamsConstraints validates request params against the constraints
// declared on a TaskAction. Returns an empty string if all constraints pass,
// or a violation reason if any constraint fails.
//
// Constraint syntax:
//
//	{"<param>": {"in":     [v1, v2, ...]}}  // value must be in the list
//	{"<param>": {"not_in": [v1, v2, ...]}}  // value must not be in the list
//	{"<param>": {"eq":     "..."}}          // value must equal
//	{"<param>": {"not_eq": "..."}}          // value must not equal
//	{"<param>": {"regex":  "^...$"}}        // value must match the regex
//
// Multiple operators on the same param are AND-combined.
//
// Semantics:
//   - String comparisons are case-insensitive (designed for emails / IDs).
//   - Array params: every element must satisfy the constraint.
//   - Param missing from the request when a constraint exists for it: violation.
//   - Malformed constraint JSON: skip silently (do not block valid requests).
//   - Malformed regex: skip that operator (do not block).
func CheckParamsConstraints(constraints json.RawMessage, params map[string]any) string {
	if len(constraints) == 0 {
		return ""
	}
	var c map[string]map[string]any
	if err := json.Unmarshal(constraints, &c); err != nil {
		return ""
	}
	for paramName, ops := range c {
		actual, present := params[paramName]
		if !present {
			return fmt.Sprintf("param %q is required by task scope but missing from request", paramName)
		}
		if reason := checkSingleParam(paramName, actual, ops); reason != "" {
			return reason
		}
	}
	return ""
}

func checkSingleParam(paramName string, actual any, ops map[string]any) string {
	values := flattenToStrings(actual)
	for op, exp := range ops {
		switch op {
		case "in":
			allowed := toStringSet(exp)
			for _, v := range values {
				if !allowed[strings.ToLower(v)] {
					return fmt.Sprintf("param %q value %q is not in the allowed list", paramName, v)
				}
			}
		case "not_in":
			blocked := toStringSet(exp)
			for _, v := range values {
				if blocked[strings.ToLower(v)] {
					return fmt.Sprintf("param %q value %q is blocked", paramName, v)
				}
			}
		case "eq":
			want, _ := exp.(string)
			for _, v := range values {
				if !strings.EqualFold(v, want) {
					return fmt.Sprintf("param %q must equal %q (got %q)", paramName, want, v)
				}
			}
		case "not_eq":
			want, _ := exp.(string)
			for _, v := range values {
				if strings.EqualFold(v, want) {
					return fmt.Sprintf("param %q must not equal %q", paramName, want)
				}
			}
		case "regex":
			pat, _ := exp.(string)
			re, err := regexp.Compile(pat)
			if err != nil {
				continue
			}
			for _, v := range values {
				if !re.MatchString(v) {
					return fmt.Sprintf("param %q value %q does not match required pattern", paramName, v)
				}
			}
		}
	}
	return ""
}

func flattenToStrings(v any) []string {
	switch x := v.(type) {
	case string:
		return []string{x}
	case []any:
		out := make([]string, 0, len(x))
		for _, e := range x {
			if s, ok := e.(string); ok {
				out = append(out, s)
			} else {
				out = append(out, fmt.Sprint(e))
			}
		}
		return out
	case []string:
		return x
	default:
		return []string{fmt.Sprint(v)}
	}
}

func toStringSet(v any) map[string]bool {
	set := map[string]bool{}
	switch x := v.(type) {
	case []any:
		for _, e := range x {
			if s, ok := e.(string); ok {
				set[strings.ToLower(s)] = true
			}
		}
	case []string:
		for _, s := range x {
			set[strings.ToLower(s)] = true
		}
	case string:
		set[strings.ToLower(x)] = true
	}
	return set
}
