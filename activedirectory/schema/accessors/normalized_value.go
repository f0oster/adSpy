package accessors

import "fmt"

// used to unwrap single-valued strings, produces an error when the slice is not single-valued
func (v *NormalizedValue) AsString() (string, error) {
	strs := v.Values
	if len(strs) == 0 {
		return "", nil
	}
	if len(strs) > 1 {
		return "", fmt.Errorf("AsString() requires a single-valued attribute, but got %d values", len(strs))
	}
	return strs[0], nil
}

// returns the full string slice
func (v *NormalizedValue) AsStringSlice() ([]string, error) {
	strs := v.Values
	if len(strs) == 0 {
		return nil, nil
	}
	return strs, nil
}

// returns the first string in the slice
func (v *NormalizedValue) FirstStringInSlice() (string, error) {
	strs := v.Values
	if len(strs) == 0 {
		return "", fmt.Errorf("slice was empty!")
	}
	return strs[0], nil
}

// returns the last string in the slice
func (v *NormalizedValue) LastStringInSlice() (string, error) {
	strs := v.Values
	if len(strs) == 0 {
		return "", fmt.Errorf("slice was empty!")
	}
	return strs[len(strs)-1], nil
}
