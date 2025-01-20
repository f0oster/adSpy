package activedirectory

import (
	"f0oster/adspy/activedirectory/formatters"
	"fmt"
)

const (
	NONE = iota
	SID_TO_STRING
)

// Active Directory attribute field type
// TODO: Formatters for string/console output?
type AttributeFieldType struct {
	GoNativeType    string
	TransformMethod formatters.Transformer
	SyntaxName      string
}

// TODO: implement?
type ObjectClass struct {
}

// TODO: implement?
type ObjectCategory struct {
}

func NewAttributeFieldType(goNativeType string, transformMethod formatters.Transformer, syntaxName string) *AttributeFieldType {
	return &AttributeFieldType{
		GoNativeType:    goNativeType,
		TransformMethod: transformMethod,
		SyntaxName:      syntaxName,
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
			"1": *NewAttributeFieldType("bool", formatters.BoolTransformer{}, "Boolean"),
		},
		"2.5.5.9": {
			"2":  *NewAttributeFieldType("string", formatters.StringTransformer{}, "Integer"),
			"10": *NewAttributeFieldType("string", formatters.StringTransformer{}, "Enumeration/Enumeration(Delivery-Mechanism)/Enumeration(Export-Information-Level)/Enumeration(Preferred-Delivery-Method)"),

			// "2":  *NewAttributeFieldType("int", formatters.IntTransformer{}, "Integer"),
			// "10": *NewAttributeFieldType("int", formatters.IntTransformer{}, "Enumeration/Enumeration(Delivery-Mechanism)/Enumeration(Export-Information-Level)/Enumeration(Preferred-Delivery-Method)"),
		},
		"2.5.5.16": {
			// "65": *NewAttributeFieldType("int64", formatters.Int64Transformer{}, "Interval/Large Integer (ADSTYPE_LARGE_INTEGER)"),
			"65": *NewAttributeFieldType("string", formatters.StringTransformer{}, "Interval/Large Integer (ADSTYPE_LARGE_INTEGER)"),
		},

		"2.5.5.13": {
			"127": *NewAttributeFieldType("string", formatters.StringTransformer{}, "Object(Presentation-Address)"),
		},

		"2.5.5.14": {
			"127": *NewAttributeFieldType("string", formatters.StringTransformer{}, "Object(Access-Point)/Object(DN-String)"),
		},

		"2.5.5.7": {
			"127": *NewAttributeFieldType("string", formatters.StringTransformer{}, "Object(DN-Binary)/Object(OR-Name)"), // possibly should be a []byte?
		},

		"2.5.5.1": {
			"127": *NewAttributeFieldType("string", formatters.StringTransformer{}, "Object(DS-DN)"),
		},

		"2.5.5.10": {
			"4":   *NewAttributeFieldType("[]byte", formatters.ByteTransformer{}, "String(Octet)"),
			"127": *NewAttributeFieldType("[]byte", formatters.ByteTransformer{}, "Object(Replica-Link)"),
		},

		"2.5.5.11": {
			"23": *NewAttributeFieldType("string", formatters.StringTransformer{}, "String(UTC-Time)"),
			"24": *NewAttributeFieldType("string", formatters.StringTransformer{}, "String(Generalized-Time)"),
			// "24": *NewAttributeFieldType("time.Time", formatters.TimeTransformer{Layout: "20060102150405.0Z"}, "String(Generalized-Time)"),
		},

		"2.5.5.5": {
			"19": *NewAttributeFieldType("string", formatters.StringTransformer{}, "String(Printable)"),
			"22": *NewAttributeFieldType("string", formatters.StringTransformer{}, "String(IA5)"),
		},

		"2.5.5.15": {
			"66": *NewAttributeFieldType("[]byte", formatters.ByteTransformer{}, "String(NT-Sec-Desc)"),
		},

		"2.5.5.6": {
			"18": *NewAttributeFieldType("string", formatters.StringTransformer{}, "String(Numeric)"),
		},

		"2.5.5.2": {
			"6": *NewAttributeFieldType("string", formatters.StringTransformer{}, "String(Object-Identifier)"),
		},

		"2.5.5.17": {
			"4": *NewAttributeFieldType("[]byte", formatters.ByteTransformer{}, "String(Sid)"),
		},

		"2.5.5.4": {
			"20": *NewAttributeFieldType("string", formatters.StringTransformer{}, "String(Teletex)"),
		},

		"2.5.5.12": {
			"64": *NewAttributeFieldType("string", formatters.StringTransformer{}, "String(Unicode)"),
		},
	}

	if innerMap, exists := typeMap[attributeSyntax]; exists {
		if goType, exists := innerMap[oMSyntax]; exists {
			return &goType, nil
		}
	}

	return nil, fmt.Errorf("error: type does not exist! AttributeSyntax: %s, oMSyntax: %s", attributeSyntax, oMSyntax)

}
