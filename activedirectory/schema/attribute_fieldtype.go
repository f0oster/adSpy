package schema

import (
	"reflect"
)

func NewAttributeFieldType(goNativeType reflect.Type, syntaxName string) *AttributeFieldType {
	return &AttributeFieldType{
		GoType:     goNativeType,
		SyntaxName: syntaxName,
	}
}
