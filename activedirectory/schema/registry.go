package schema

import (
	"fmt"
	"reflect"
	"time"

	"f0oster/adspy/activedirectory/transformers"

	"github.com/f0oster/gontsd"
	"github.com/google/uuid"
)

type SchemaRegistry struct {
	attributeSchemas map[string]*AttributeSchema
	typeMap          map[string]map[string]*AttributeFieldType // syntax → omsyntax
	attributeHooks   map[string]*AttributeFieldType            // ldapDisplayName → handler
}

func NewSchemaRegistry() *SchemaRegistry {
	r := &SchemaRegistry{
		typeMap:          make(map[string]map[string]*AttributeFieldType),
		attributeHooks:   make(map[string]*AttributeFieldType),
		attributeSchemas: make(map[string]*AttributeSchema),
	}
	r.init()
	return r
}

func (r *SchemaRegistry) Register(attributeSyntax, oMSyntax string, goType reflect.Type, normalizer transformers.Normalizer, interpreter transformers.Interpreter, syntaxName string) {
	if _, ok := r.typeMap[attributeSyntax]; !ok {
		r.typeMap[attributeSyntax] = make(map[string]*AttributeFieldType)
	}
	r.typeMap[attributeSyntax][oMSyntax] = &AttributeFieldType{
		GoType:      goType,
		SyntaxName:  syntaxName,
		Normalizer:  normalizer,
		Interpreter: interpreter,
	}
}

func (r *SchemaRegistry) OverrideAttribute(ldapName string, fieldType *AttributeFieldType) {
	r.attributeHooks[ldapName] = fieldType
}

func (r *SchemaRegistry) Lookup(attributeSyntax, oMSyntax, ldapName string) (*AttributeFieldType, error) {
	if ft, ok := r.attributeHooks[ldapName]; ok {
		return ft, nil
	}
	if omMap, ok := r.typeMap[attributeSyntax]; ok {
		if ft, ok := omMap[oMSyntax]; ok {
			return ft, nil
		}
	}
	return nil, fmt.Errorf("no type mapping for syntax=%s oMSyntax=%s ldapName=%s", attributeSyntax, oMSyntax, ldapName)
}

func (r *SchemaRegistry) GetAttributeSchema(ldapName string) (*AttributeSchema, bool) {
	schema, ok := r.attributeSchemas[ldapName]
	return schema, ok
}

func (r *SchemaRegistry) RegisterAttributeSchema(schema *AttributeSchema) {
	r.attributeSchemas[schema.AttributeLDAPName] = schema
}

func (r *SchemaRegistry) registerSchemaSyntax() {
	// Boolean
	r.Register("2.5.5.8", "1", reflect.TypeOf(true), transformers.SimpleStringFormatter{}, transformers.SimpleStringFormatter{}, "Boolean")

	// Integer types
	r.Register("2.5.5.9", "2", reflect.TypeOf(int(0)), transformers.SimpleStringFormatter{}, transformers.SimpleStringFormatter{}, "Integer")
	r.Register("2.5.5.9", "10", reflect.TypeOf(int(0)), transformers.SimpleStringFormatter{}, transformers.SimpleStringFormatter{}, "Enumeration")

	// Large Integer (as int64)
	r.Register("2.5.5.16", "65", reflect.TypeOf((*time.Time)(nil)), transformers.ADFiletimeFormatter{}, transformers.ADFiletimeFormatter{}, "Large Integer (FILETIME)")

	// String representations
	r.Register("2.5.5.13", "127", reflect.TypeOf(""), transformers.SimpleStringFormatter{}, transformers.SimpleStringFormatter{}, "Presentation Address")
	r.Register("2.5.5.14", "127", reflect.TypeOf(""), transformers.SimpleStringFormatter{}, transformers.SimpleStringFormatter{}, "Access Point / DN-String")
	r.Register("2.5.5.7", "127", reflect.TypeOf(""), transformers.SimpleStringFormatter{}, transformers.SimpleStringFormatter{}, "DN-Binary / OR-Name")
	r.Register("2.5.5.1", "127", reflect.TypeOf(""), transformers.SimpleStringFormatter{}, transformers.SimpleStringFormatter{}, "DS-DN")
	r.Register("2.5.5.5", "19", reflect.TypeOf(""), transformers.SimpleStringFormatter{}, transformers.SimpleStringFormatter{}, "Printable String")
	r.Register("2.5.5.5", "22", reflect.TypeOf(""), transformers.SimpleStringFormatter{}, transformers.SimpleStringFormatter{}, "IA5 String")
	r.Register("2.5.5.6", "18", reflect.TypeOf(""), transformers.SimpleStringFormatter{}, transformers.SimpleStringFormatter{}, "Numeric String")
	r.Register("2.5.5.2", "6", reflect.TypeOf(""), transformers.SimpleStringFormatter{}, transformers.SimpleStringFormatter{}, "Object Identifier")
	r.Register("2.5.5.4", "20", reflect.TypeOf(""), transformers.SimpleStringFormatter{}, transformers.SimpleStringFormatter{}, "Teletex String")
	r.Register("2.5.5.12", "64", reflect.TypeOf(""), transformers.SimpleStringFormatter{}, transformers.SimpleStringFormatter{}, "Unicode String")

	// Octet / Binary blobs
	r.Register("2.5.5.10", "4", reflect.TypeOf([]byte{}), transformers.Base64Formatter{}, transformers.SimpleStringFormatter{}, "Octet String")
	r.Register("2.5.5.10", "127", reflect.TypeOf([]byte{}), transformers.SimpleStringFormatter{}, transformers.SimpleStringFormatter{}, "Replica-Link")

	// Time
	r.Register("2.5.5.11", "23", reflect.TypeOf(""), transformers.SimpleStringFormatter{}, transformers.SimpleStringFormatter{}, "UTC-Time") // kept as string
	r.Register("2.5.5.11", "24", reflect.TypeOf(time.Time{}), transformers.LDAPTimeFormatter{
		Layout: "20060102150405.0Z",
	}, transformers.SimpleStringFormatter{}, "Generalized-Time")

	// Security descriptor and SID
	r.Register("2.5.5.15", "66", reflect.TypeOf(&gontsd.SecurityDescriptor{}), transformers.NTSecurityDescriptorFormatter{}, transformers.NTSecurityDescriptorFormatter{}, "NT-Sec-Desc")
	r.Register("2.5.5.17", "4", reflect.TypeOf([]byte{}), transformers.SimpleStringFormatter{}, transformers.SimpleStringFormatter{}, "SID")

}

func (r *SchemaRegistry) registerAttributeOverrides() {
	r.OverrideAttribute("uSNCreated", &AttributeFieldType{
		GoType:      reflect.TypeOf(int64(0)),
		SyntaxName:  "Large Integer",
		Interpreter: transformers.SimpleStringFormatter{},
		Normalizer:  transformers.SimpleStringFormatter{},
	})

	r.OverrideAttribute("uSNChanged", &AttributeFieldType{
		GoType:      reflect.TypeOf(int64(0)),
		SyntaxName:  "Large Integer",
		Interpreter: transformers.SimpleStringFormatter{},
		Normalizer:  transformers.SimpleStringFormatter{},
	})

	r.OverrideAttribute("objectGUID", &AttributeFieldType{
		GoType:      reflect.TypeOf([]uuid.UUID{}),
		SyntaxName:  "Octet String",
		Interpreter: transformers.ADGuidFormatter{},
		Normalizer:  transformers.ADGuidFormatter{},
	})

	r.OverrideAttribute("objectSid", &AttributeFieldType{
		GoType:      reflect.TypeOf(""),
		SyntaxName:  "SID",
		Interpreter: transformers.SIDFormatter{},
		Normalizer:  transformers.SIDFormatter{},
	})

	r.OverrideAttribute("tokenGroups", &AttributeFieldType{
		GoType:      reflect.TypeOf(""),
		SyntaxName:  "SID",
		Interpreter: transformers.SIDFormatter{},
		Normalizer:  transformers.SIDFormatter{},
	})
}

func (r *SchemaRegistry) init() {
	// See MS documentation: https://learn.microsoft.com/en-us/windows/win32/adschema/syntaxes
	r.registerSchemaSyntax()
	r.registerAttributeOverrides()
}
