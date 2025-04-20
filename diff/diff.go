package diff

// FindChanges compares two attribute snapshots and returns a list of changes
func FindChanges(prev, curr map[string]interface{}) []AttributeChange {
	var changes []AttributeChange

	// Detect changed or added attributes
	for k, newVal := range curr {
		oldVal, exists := prev[k]
		if !exists || !compareAsStringOrSlice(oldVal, newVal) {
			changes = append(changes, AttributeChange{
				Name: k,
				Old:  oldVal,
				New:  newVal,
			})
		}
	}

	// Detect removed attributes
	for k, oldVal := range prev {
		if _, exists := curr[k]; !exists {
			changes = append(changes, AttributeChange{
				Name: k,
				Old:  oldVal,
				New:  nil,
			})
		}
	}

	return changes
}

func compareAsStringOrSlice(a, b interface{}) bool {
	aslice, err := AssertStringSlice(a)
	if err != nil {
		panic("AssertStringSlice failed - aslice did not contain a string or a string slice")
	}

	bslice, err := AssertStringSlice(b)
	if err != nil {
		panic("AssertStringSlice failed - bslice did not contain a string or a string slice")
	}

	if len(aslice) != len(bslice) {
		return false
	}

	for i := range aslice {
		if aslice[i] != bslice[i] {
			return false
		}
	}
	return true
}
