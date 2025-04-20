package diff

import "fmt"

// flattenToStrings ensures consistent comparison by flattening scalars or slices to []string
func AssertStringSlice(v interface{}) ([]string, error) {
	switch val := v.(type) {
	case string:
		return []string{val}, nil

	case []string:
		return val, nil

	case []interface{}:
		result := make([]string, len(val))
		for i, item := range val {
			str, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("expected string at index %d, got %T", i, item)
			}
			result[i] = str
		}
		return result, nil

	default:
		return nil, fmt.Errorf("expected string or []interface{} of strings, got %T", v)
	}
}
