package schema

import (
	"f0oster/adspy/activedirectory/schema/accessors"
)

// NewAttributeValue constructs a new AttributeValue with schema metadata and parsed values.
func NewAttributeValue(
	name string,
	normalized *accessors.NormalizedValue,
	interpreted *accessors.InterpretedValue,
	ldapRawValues []string,
	ldapByteValues [][]byte,
	schema *AttributeSchema,
) *AttributeValue {
	return &AttributeValue{
		Name:             name,
		NormalizedValue:  normalized,
		InterpretedValue: interpreted,
		LDAPRawValue:     ldapRawValues,
		LDAPByteValue:    ldapByteValues,
		Schema:           schema,
	}
}

// AsString returns the normalized value as a single string.
func (a *AttributeValue) AsString() (string, error) {
	return a.NormalizedValue.AsString()
}

// AsStringSlice returns the normalized value as a string slice.
func (a *AttributeValue) AsStringSlice() ([]string, error) {
	return a.NormalizedValue.AsStringSlice()
}
