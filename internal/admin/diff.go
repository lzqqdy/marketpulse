package admin

import (
	"fmt"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

// DiffTrees compares live vs example maps and returns structural gaps.
func DiffTrees(live, example map[string]any) []Gap {
	var gaps []Gap
	walkDiff("", live, example, &gaps)
	// also note keys only in live (shallow walk for extras)
	walkExtras("", live, example, &gaps)
	return gaps
}

func walkDiff(prefix string, live, example map[string]any, gaps *[]Gap) {
	for key, exVal := range example {
		path := joinPath(prefix, key)
		liveVal, ok := live[key]
		if !ok {
			*gaps = append(*gaps, Gap{
				Path:                 path,
				Kind:                 "missing_in_live",
				ExampleValue:         exVal,
				SuggestedYAMLSnippet: formatSnippet(key, exVal),
			})
			continue
		}
		exMap, exIsMap := exVal.(map[string]any)
		liveMap, liveIsMap := liveVal.(map[string]any)
		if exIsMap && liveIsMap {
			walkDiff(path, liveMap, exMap, gaps)
			continue
		}
		if exIsMap != liveIsMap || kindTag(exVal) != kindTag(liveVal) {
			// treat list vs scalar etc as mismatch; skip nested map already handled
			if !sameContainerKind(exVal, liveVal) {
				*gaps = append(*gaps, Gap{
					Path:                 path,
					Kind:                 "type_mismatch",
					ExampleValue:         exVal,
					LiveValue:            liveVal,
					SuggestedYAMLSnippet: formatSnippet(key, exVal),
				})
			}
		}
	}
}

func walkExtras(prefix string, live, example map[string]any, gaps *[]Gap) {
	for key, liveVal := range live {
		path := joinPath(prefix, key)
		exVal, ok := example[key]
		if !ok {
			*gaps = append(*gaps, Gap{
				Path:      path,
				Kind:      "missing_in_example",
				LiveValue: liveVal,
			})
			continue
		}
		liveMap, liveIsMap := liveVal.(map[string]any)
		exMap, exIsMap := exVal.(map[string]any)
		if liveIsMap && exIsMap {
			walkExtras(path, liveMap, exMap, gaps)
		}
	}
}

func joinPath(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

func kindTag(v any) string {
	if v == nil {
		return "null"
	}
	switch v.(type) {
	case map[string]any:
		return "map"
	case []any:
		return "list"
	case bool:
		return "bool"
	case int, int64, uint64, float64, float32:
		return "number"
	case string:
		return "string"
	default:
		return reflect.TypeOf(v).String()
	}
}

func sameContainerKind(a, b any) bool {
	_, aMap := a.(map[string]any)
	_, bMap := b.(map[string]any)
	if aMap || bMap {
		return aMap && bMap
	}
	_, aList := a.([]any)
	_, bList := b.([]any)
	if aList || bList {
		return aList && bList
	}
	return true
}

func formatSnippet(key string, val any) string {
	var buf strings.Builder
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	_ = enc.Encode(map[string]any{key: val})
	_ = enc.Close()
	return strings.TrimSpace(buf.String())
}

// ApplyGapValue sets path on tree to value (creates intermediate maps).
func ApplyGapValue(tree map[string]any, path string, value any) error {
	parts := strings.Split(path, ".")
	if len(parts) == 0 || parts[0] == "" {
		return fmt.Errorf("empty path")
	}
	cur := tree
	for i, p := range parts {
		if i == len(parts)-1 {
			cur[p] = value
			return nil
		}
		next, ok := cur[p]
		if !ok {
			m := map[string]any{}
			cur[p] = m
			cur = m
			continue
		}
		m, ok := next.(map[string]any)
		if !ok {
			return fmt.Errorf("path %s conflicts with non-object", strings.Join(parts[:i+1], "."))
		}
		cur = m
	}
	return nil
}
