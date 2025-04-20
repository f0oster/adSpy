package schema

import (
	"f0oster/adspy/activedirectory/transformers"
	"fmt"
	"reflect"
)

// AttributeSchema holds schema information for directory attributes
type AttributeSchema struct {
	AttributeName           string
	AttributeLDAPName       string
	AttributeID             string
	AttributeSyntax         string
	AttributeOMSyntax       string
	AttributeFieldType      AttributeFieldType
	AttributeIsSingleValued bool
}

type AttributeValue struct {
	Name             string           `json:"name"`
	Schema           *AttributeSchema `json:"schema"`
	LDAPRawValue     interface{}
	LDAPByteValue    [][]byte
	NormalizedValue  *NormalizedValue `json:"normalized_value"`
	InterpretedValue *InterpretedValue
}

func As[T any](attr *AttributeValue) (T, error) {
	if len(attr.InterpretedValue.Values) == 0 {
		var zero T
		return zero, nil
	}

	val := attr.InterpretedValue.Values[0]
	cast, ok := val.(T)
	if !ok {
		var zero T
		return zero, fmt.Errorf("expected %T, got %T", zero, val)
	}
	return cast, nil
}

func AsSlice[T any](attr *AttributeValue) ([]T, error) {
	var result []T
	for i, raw := range attr.InterpretedValue.Values {
		val, ok := raw.(T)
		if !ok {
			return nil, fmt.Errorf("at index %d: expected %T, got %T", i, *new(T), raw)
		}
		result = append(result, val)
	}
	return result, nil
}

func (a *AttributeValue) AsString() (string, error) {
	return a.NormalizedValue.AsString()
}

func (a *AttributeValue) AsStringSlice() ([]string, error) {
	return a.NormalizedValue.AsStringSlice()
}

// Active Directory attribute field type
type AttributeFieldType struct {
	GoType        reflect.Type
	SyntaxName    string
	Normalizer    transformers.Normalizer
	Interpreter   transformers.Interpreter
	CustomHandler func(attr *AttributeValue) error
}

func NewAttributeFieldType(goNativeType reflect.Type, syntaxName string) *AttributeFieldType {
	return &AttributeFieldType{
		GoType:     goNativeType,
		SyntaxName: syntaxName,
	}
}

type NormalizedValue struct {
	Values []string
}

type InterpretedValue struct {
	Values []interface{}
}

func (v InterpretedValue) First() interface{} {
	if len(v.Values) > 0 {
		return v.Values[0]
	}
	return nil
}

func (v NormalizedValue) AsString() (string, error) {
	strs := v.Values
	if len(strs) == 0 {
		return "", nil
	}
	if len(strs) > 1 {
		return "", fmt.Errorf("AsString() requires a single-valued attribute, but got %d values", len(strs))
	}
	return strs[0], nil
}

func (v NormalizedValue) AsStringSlice() ([]string, error) {
	strs := v.Values
	if len(strs) == 0 {
		return nil, nil
	}
	return strs, nil
}
