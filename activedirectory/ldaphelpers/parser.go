package ldaphelpers

import (
	"f0oster/adspy/activedirectory/schema"
	"f0oster/adspy/activedirectory/schema/accessors"
	"fmt"
	"log"

	"github.com/go-ldap/ldap/v3"
)

func ParseAttribute(
	attr *ldap.EntryAttribute,
	attributeSchema schema.AttributeSchema,
) (*schema.AttributeValue, error) {
	byteValues := attr.ByteValues

	fieldType := attributeSchema.AttributeFieldType
	if fieldType.Normalizer == nil {
		return nil, fmt.Errorf("normalizer not defined for attribute %s", attributeSchema.AttributeLDAPName)
	}

	// --- Normalize ---
	normalizedStrings, normErr := fieldType.Normalizer.Normalize(byteValues)
	if normErr != nil {
		log.Printf("Failed to normalize %s: %v", attr.Name, normErr)
	}
	normalized := &accessors.NormalizedValue{Values: normalizedStrings}

	log.Printf("%s %v (%s / %s / %s / SingleValued: %v): %s",

		attr.Name,
		normalizedStrings,
		attributeSchema.AttributeFieldType.SyntaxName,
		attributeSchema.AttributeSyntax,
		attributeSchema.AttributeOMSyntax,
		attributeSchema.AttributeIsSingleValued,
		attributeSchema.AttributeFieldType.GoType.String(),
	)

	// --- Interpret ---
	interpreted := &accessors.InterpretedValue{}
	interpretedVal, interpErr := fieldType.Interpreter.Interpret(byteValues)
	if interpErr != nil {
		log.Printf("Failed to interpret %s: %v", attr.Name, interpErr)
	} else {
		switch v := interpretedVal.(type) {
		case []interface{}:
			interpreted.Values = v
		default:
			interpreted.Values = []interface{}{v} // wrap the result in a slice
		}
	}

	return schema.NewAttributeValue(
		attr.Name,
		normalized,
		interpreted,
		attr.Values,
		attr.ByteValues,
		&attributeSchema,
	), nil
}
