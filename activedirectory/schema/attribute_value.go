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
