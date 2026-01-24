package schema

import (
	"reflect"

	"f0oster/adspy/activedirectory/schema/accessors"
	"f0oster/adspy/activedirectory/transformers"

	"github.com/google/uuid"
)

type AttributeFieldType struct {
	GoType      reflect.Type
	SyntaxName  string
	Normalizer  transformers.Normalizer
	Interpreter transformers.Interpreter
}

type AttributeSchema struct {
	ObjectGUID              uuid.UUID
	AttributeName           string
	AttributeLDAPName       string
	AttributeID             string
	AttributeSyntax         string
	AttributeOMSyntax       string
	AttributeFieldType      AttributeFieldType
	AttributeIsSingleValued bool
}

// AttributeValue represents a runtime-loaded AD attribute for a specific object.
// It includes normalized (string-friendly) and interpreted (Go-native) forms.
type AttributeValue struct {
	Name             string                      `json:"name"`
	Schema           *AttributeSchema            `json:"schema"`
	LDAPRawValue     interface{}                 `json:"ldap_raw_value"`
	LDAPByteValue    [][]byte                    `json:"ldap_byte_value"`
	NormalizedValue  *accessors.NormalizedValue  `json:"normalized_value"`
	InterpretedValue *accessors.InterpretedValue `json:"-"`
}
