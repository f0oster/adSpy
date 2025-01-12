package activedirectory

import (
	"fmt"
)

const (
	NONE = iota
	SID_TO_STRING
)

// Active Directory attribute field type
type AttributeFieldType struct {
	GoNativeType       string
	CustomRenderFormat int
	TransformMethod    int
	SyntaxName         string
}

func NewAttributeFieldType(goNativeType string, customRenderFormat int, transformMethod int, syntaxName string) *AttributeFieldType {
	return &AttributeFieldType{
		GoNativeType:       goNativeType,
		CustomRenderFormat: customRenderFormat,
		TransformMethod:    transformMethod,
		SyntaxName:         syntaxName,
	}
}

// AttributeSchema holds schema information for directory attributes
type AttributeSchema struct {
	AttributeName      string
	AttributeLDAPName  string
	AttributeID        string
	AttributeSyntax    string
	AttributeOMSyntax  string
	AttributeFieldType AttributeFieldType
}

// https://learn.microsoft.com/en-us/windows/win32/adschema/syntaxes
// Map the passed in attribute type to an AttributeFieldType
func mapSchemaToTypes(attributeSyntax, oMSyntax string) (*AttributeFieldType, error) {
	typeMap := map[string]map[string]AttributeFieldType{
		"2.5.5.8": {
			"1": *NewAttributeFieldType("bool", NONE, NONE, "Boolean"),
		},
		"2.5.5.9": {
			"2":  *NewAttributeFieldType("int", NONE, NONE, "Integer"),
			"10": *NewAttributeFieldType("int", NONE, NONE, "Enumeration/Enumeration(Delivery-Mechanism)/Enumeration(Export-Information-Level)/Enumeration(Preferred-Delivery-Method)"),
		},
		"2.5.5.16": {
			"65": *NewAttributeFieldType("int64", NONE, NONE, "Interval/Large Integer (ADSTYPE_LARGE_INTEGER)"),
		},

		"2.5.5.13": {
			"127": *NewAttributeFieldType("string", NONE, NONE, "Object(Presentation-Address)"),
		},

		"2.5.5.14": {
			"127": *NewAttributeFieldType("string", NONE, NONE, "Object(Access-Point)/Object(DN-String)"),
		},

		"2.5.5.7": {
			"127": *NewAttributeFieldType("string", NONE, NONE, "Object(DN-Binary)/Object(OR-Name)"), // possibly should be a []byte?
		},

		"2.5.5.1": {
			"127": *NewAttributeFieldType("string", NONE, NONE, "Object(DS-DN)"),
		},

		"2.5.5.10": {
			"4":   *NewAttributeFieldType("[]byte", NONE, NONE, "String(Octet)"),
			"127": *NewAttributeFieldType("[]byte", NONE, NONE, "Object(Replica-Link)"),
		},

		"2.5.5.11": {
			"23": *NewAttributeFieldType("string", NONE, NONE, "String(UTC-Time)"),
			"24": *NewAttributeFieldType("time.Time", NONE, NONE, "String(Generalized-Time)"),
		},

		"2.5.5.5": {
			"19": *NewAttributeFieldType("string", NONE, NONE, "String(Printable)"),
			"22": *NewAttributeFieldType("string", NONE, NONE, "String(IA5)"),
		},

		"2.5.5.15": {
			"66": *NewAttributeFieldType("[]byte", NONE, NONE, "String(NT-Sec-Desc)"),
		},

		"2.5.5.6": {
			"18": *NewAttributeFieldType("string", NONE, NONE, "String(Numeric)"),
		},

		"2.5.5.2": {
			"6": *NewAttributeFieldType("string", NONE, NONE, "String(Object-Identifier)"),
		},

		"2.5.5.17": {
			"4": *NewAttributeFieldType("[]byte", SID_TO_STRING, NONE, "String(Sid)"),
		},

		"2.5.5.4": {
			"20": *NewAttributeFieldType("string", NONE, NONE, "String(Teletex)"),
		},

		"2.5.5.12": {
			"64": *NewAttributeFieldType("string", NONE, NONE, "String(Unicode)"),
		},
	}

	if innerMap, exists := typeMap[attributeSyntax]; exists {
		if goType, exists := innerMap[oMSyntax]; exists {
			return &goType, nil
		}
	}

	return nil, fmt.Errorf("error: type does not exist! AttributeSyntax: %s, oMSyntax: %s", attributeSyntax, oMSyntax)

}
