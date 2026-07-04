package codex

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

func serializeConfigOverrides(config ConfigObject) ([]string, error) {
	if len(config) == 0 {
		return nil, nil
	}
	var overrides []string
	if err := flattenConfigOverrides(config, "", &overrides); err != nil {
		return nil, err
	}
	return overrides, nil
}

func flattenConfigOverrides(value any, prefix string, overrides *[]string) error {
	obj, ok := asStringMap(value)
	if !ok {
		if prefix == "" {
			return errors.New("codex config overrides must be a plain object")
		}
		rendered, err := toTOMLLiteral(value, prefix)
		if err != nil {
			return err
		}
		*overrides = append(*overrides, prefix+"="+rendered)
		return nil
	}

	keys := make([]string, 0, len(obj))
	for key := range obj {
		if key == "" {
			return errors.New("codex config override keys must be non-empty strings")
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	if prefix != "" && len(keys) == 0 {
		*overrides = append(*overrides, prefix+"={}")
		return nil
	}

	for _, key := range keys {
		child := obj[key]
		if child == nil {
			return fmt.Errorf("codex config override at %s cannot be null", key)
		}
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}
		if _, ok := asStringMap(child); ok {
			if err := flattenConfigOverrides(child, path, overrides); err != nil {
				return err
			}
			continue
		}
		rendered, err := toTOMLLiteral(child, path)
		if err != nil {
			return err
		}
		*overrides = append(*overrides, path+"="+rendered)
	}
	return nil
}

func toTOMLLiteral(value any, path string) (string, error) {
	switch v := value.(type) {
	case string:
		return quoteString(v), nil
	case bool:
		if v {
			return "true", nil
		}
		return "false", nil
	case int:
		return strconv.Itoa(v), nil
	case int8, int16, int32, int64:
		return fmt.Sprintf("%d", reflect.ValueOf(v).Int()), nil
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", reflect.ValueOf(v).Uint()), nil
	case float32:
		f := float64(v)
		if !isFinite(f) {
			return "", fmt.Errorf("codex config override at %s must be a finite number", path)
		}
		return strconv.FormatFloat(f, 'f', -1, 32), nil
	case float64:
		if !isFinite(v) {
			return "", fmt.Errorf("codex config override at %s must be a finite number", path)
		}
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	case nil:
		return "", fmt.Errorf("codex config override at %s cannot be null", path)
	}

	if obj, ok := asStringMap(value); ok {
		keys := make([]string, 0, len(obj))
		for key := range obj {
			if key == "" {
				return "", errors.New("codex config override keys must be non-empty strings")
			}
			keys = append(keys, key)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, key := range keys {
			child := obj[key]
			rendered, err := toTOMLLiteral(child, path+"."+key)
			if err != nil {
				return "", err
			}
			parts = append(parts, formatTOMLKey(key)+" = "+rendered)
		}
		return "{" + strings.Join(parts, ", ") + "}", nil
	}

	rv := reflect.ValueOf(value)
	if rv.IsValid() && (rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array) {
		parts := make([]string, 0, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			rendered, err := toTOMLLiteral(rv.Index(i).Interface(), fmt.Sprintf("%s[%d]", path, i))
			if err != nil {
				return "", err
			}
			parts = append(parts, rendered)
		}
		return "[" + strings.Join(parts, ", ") + "]", nil
	}

	return "", fmt.Errorf("unsupported codex config override value at %s: %T", path, value)
}

func asStringMap(value any) (map[string]any, bool) {
	switch v := value.(type) {
	case ConfigObject:
		result := make(map[string]any, len(v))
		for key, child := range v {
			result[key] = child
		}
		return result, true
	case map[string]any:
		return v, true
	default:
		return nil, false
	}
}

func quoteString(value string) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func formatTOMLKey(key string) string {
	for _, r := range key {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			continue
		}
		return quoteString(key)
	}
	return key
}

func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
