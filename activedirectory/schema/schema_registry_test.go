package schema_test

import (
	"reflect"
	"testing"
	"time"

	"f0oster/adspy/activedirectory/schema"
	"f0oster/adspy/activedirectory/transformers"

	"github.com/f0oster/gontsd"
	"github.com/google/uuid"
)

func TestSchemaRegistry_Lookup_BuiltIn(t *testing.T) {
	r := schema.NewSchemaRegistry()

	type testCase struct {
		attributeSyntax string
		oMSyntax        string
		ldapName        string
		expectedType    reflect.Type
		expectedSyntax  string
	}

	tests := []testCase{
		{"2.5.5.8", "1", "someBoolean", reflect.TypeOf(true), "Boolean"},
		{"2.5.5.9", "2", "someInteger", reflect.TypeOf(int(0)), "Integer"},
		{"2.5.5.16", "65", "whenCreated", reflect.TypeOf((*time.Time)(nil)), "Large Integer (FILETIME)"},
		{"2.5.5.15", "66", "ntSecurityDescriptor", reflect.TypeOf(&gontsd.SecurityDescriptor{}), "NT-Sec-Desc"},
	}

	for _, test := range tests {
		fieldType, err := r.Lookup(test.attributeSyntax, test.oMSyntax, test.ldapName)
		if err != nil {
			t.Fatalf("Lookup failed for %s/%s: %v", test.attributeSyntax, test.oMSyntax, err)
		}
		if fieldType.GoType != test.expectedType {
			t.Errorf("Unexpected GoType for %s: got %v, want %v", test.ldapName, fieldType.GoType, test.expectedType)
		}
		if fieldType.SyntaxName != test.expectedSyntax {
			t.Errorf("Unexpected SyntaxName for %s: got %s, want %s", test.ldapName, fieldType.SyntaxName, test.expectedSyntax)
		}
	}
}

func TestSchemaRegistry_Lookup_Override(t *testing.T) {
	r := schema.NewSchemaRegistry()

	fieldType, err := r.Lookup("", "", "objectGUID")
	if err != nil {
		t.Fatalf("Expected override for objectGUID, got error: %v", err)
	}
	if fieldType.GoType != reflect.TypeOf([]uuid.UUID{}) {
		t.Errorf("Unexpected GoType for objectGUID: got %v", fieldType.GoType)
	}
	if _, ok := fieldType.Normalizer.(transformers.ADGuidFormatter); !ok {
		t.Errorf("objectGUID Normalizer is not ADGuidFormatter")
	}
	if _, ok := fieldType.Interpreter.(transformers.ADGuidFormatter); !ok {
		t.Errorf("objectGUID Interpreter is not ADGuidFormatter")
	}
}

func TestSchemaRegistry_UnknownMapping(t *testing.T) {
	r := schema.NewSchemaRegistry()

	_, err := r.Lookup("2.5.5.99", "999", "nonExistent")
	if err == nil {
		t.Errorf("Expected error for unknown mapping, got nil")
	}
}

func TestSchemaRegistry_GetAttributeSchema(t *testing.T) {
	r := schema.NewSchemaRegistry()

	schemaEntry := &schema.AttributeSchema{AttributeLDAPName: "testAttribute"}
	r.RegisterAttributeSchema(schemaEntry)

	fetched, ok := r.GetAttributeSchema("testAttribute")
	if !ok {
		t.Fatal("Expected attribute schema to be found")
	}
	if fetched != schemaEntry {
		t.Error("Fetched schema does not match registered one")
	}
}

func TestSchemaRegistry_OverrideAttribute(t *testing.T) {
	r := schema.NewSchemaRegistry()

	override := &schema.AttributeFieldType{
		GoType:      reflect.TypeOf(""),
		SyntaxName:  "OverrideTest",
		Normalizer:  transformers.SimpleStringFormatter{},
		Interpreter: transformers.SimpleStringFormatter{},
	}

	r.OverrideAttribute("customAttribute", override)

	resolved, err := r.Lookup("", "", "customAttribute")
	if err != nil {
		t.Fatalf("OverrideAttribute failed: %v", err)
	}
	if resolved.SyntaxName != "OverrideTest" {
		t.Errorf("Expected OverrideTest, got %s", resolved.SyntaxName)
	}
}

func TestSchemaRegistry_AttributeNotFound(t *testing.T) {
	r := schema.NewSchemaRegistry()

	_, ok := r.GetAttributeSchema("notRegistered")
	if ok {
		t.Errorf("Expected false for unregistered attribute schema, got true")
	}
}
