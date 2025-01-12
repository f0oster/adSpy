package ldaphelpers

import (
	"f0oster/adspy/activedirectory"
	"fmt"
	"log"
	"strings"

	"github.com/go-ldap/ldap/v3"
)

const (
	AllObjects      = "(objectClass=*)"
	AllGroupObjects = "(objectClass=group)"
	AllUserObjects  = "(&(objectCategory=person)(objectClass=user))"
)

// print the LDAP search results to console
func PrintToConsole(adInstance *activedirectory.ActiveDirectoryInstance, entries []*ldap.Entry) error {

	for _, entry := range entries {
		// Prepare a list of schemas and values for the entry
		var schemas []activedirectory.AttributeSchema
		values := make(map[string][]string)

		for _, attribute := range entry.Attributes {
			if schema, ok := adInstance.SchemaMap[attribute.Name]; ok {
				schemas = append(schemas, schema)
				values[attribute.Name] = attribute.Values
			}
		}

		// Print entry details
		// PrintEntryTree(entry.DN, schemas, values, adInstance.UnmarshalAttributeData)

		fmt.Println(strings.Repeat("─", 80)) // Horizontal separator
		fmt.Printf("DN: %s\n", entry.DN)
		fmt.Println(strings.Repeat("─", 80)) // Horizontal separator

		for i, schema := range schemas {
			// Print attribute name
			prefix := "├───"
			if i == len(schemas)-1 {
				prefix = "└───"
			}
			fmt.Printf("%sAttribute: %s (%s)\n", prefix, schema.AttributeName, schema.AttributeLDAPName)

			// Indent and print schema details
			fmt.Printf("    ├──AttributeID: %s\n", schema.AttributeID)
			fmt.Printf("    ├──AttributeSyntax: %s\n", schema.AttributeSyntax)
			fmt.Printf("    ├──AttributeOMSyntax: %s\n", schema.AttributeOMSyntax)
			fmt.Printf("    ├──SchemaSyntax: %s\n", schema.AttributeFieldType.SyntaxName)
			fmt.Printf("    ├──GoNativeType: %s\n", schema.AttributeFieldType.GoNativeType)

			// Print attribute values
			attrValues, ok := values[schema.AttributeLDAPName]
			if !ok || len(attrValues) == 0 {
				fmt.Println("    └──Value: [No values]")
				continue
			}

			for j, rawValue := range attrValues {
				value, err := adInstance.UnmarshalAttributeData(schema.AttributeLDAPName, rawValue)
				if err != nil {
					log.Printf("Error unmarshalling attribute %s: %v", schema.AttributeLDAPName, err)
					continue
				}

				// Determine if it's the last value for formatting
				valuePrefix := "    ├──Value:"
				if j == len(attrValues)-1 {
					valuePrefix = "    └──Value:"
				}
				fmt.Printf("%s %v\n", valuePrefix, value)
			}
		}

	}
	return nil
}
