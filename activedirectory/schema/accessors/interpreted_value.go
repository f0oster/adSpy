package accessors

import (
	"fmt"
	"reflect"
)

func FirstAs[T any](iv InterpretedValue) (T, error) {
	var zero T
	if len(iv.Values) == 0 {
		return zero, nil // TODO: probably should return an error?
	}
	val, ok := iv.Values[0].(T)
	if !ok {
		return zero, fmt.Errorf(
			"InterpretedValue.FirstAs[%s]: type mismatch: got %T, expected %s",
			reflect.TypeOf(zero),
			val,
			reflect.TypeOf(zero),
		)
	}
	return val, nil
}
