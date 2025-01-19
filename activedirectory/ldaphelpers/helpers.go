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

// Return a map of matched fetched single value attributes from an *ldap.Entry
func ExtractAttributes(entry *ldap.Entry, requiredAttrs []string) (map[string]string, error) {
	attrMap := make(map[string]string)
	for _, attr := range entry.Attributes {
		attrMap[strings.ToLower(attr.Name)] = attr.Values[0]
	}

	result := make(map[string]string)
	for _, attr := range requiredAttrs {
		value, exists := attrMap[strings.ToLower(attr)]
		if !exists || value == "" {
			return nil, fmt.Errorf("missing or empty attribute: %s", attr)
		}
		result[attr] = value
	}

	return result, nil
}

// marshal LDAP result into a map based on Transformer assigned to the AttributeFieldType in the AttributeSchema
func SerializeAttributes(entry *ldap.Entry, adInstance *activedirectory.ActiveDirectoryInstance) (map[string]interface{}, error) {
	attributesSnapshot := make(map[string]interface{})

	for _, attr := range entry.Attributes {
		schema, exists := adInstance.SchemaAttributeMap[attr.Name]
		if !exists {
			log.Printf("No schema found for attribute %s", attr.Name)
			continue
		}

		if schema.AttributeFieldType.TransformMethod != nil {
			transformedValue, err := schema.AttributeFieldType.TransformMethod.Transform(attr.Values)
			if err != nil {
				return nil, fmt.Errorf("failed to transform attribute %s: %w", attr.Name, err)
			}
			attributesSnapshot[attr.Name] = transformedValue
		} else {
			// Handle attributes without a transformer
			log.Printf("Serializer falling back to raw value - no mapped transformer for: %s", attr.Name)
			attributesSnapshot[attr.Name] = attr.Values
		}
	}

	return attributesSnapshot, nil
}

// print the LDAP search results to console
func PrintToConsole(adInstance *activedirectory.ActiveDirectoryInstance, entries []*ldap.Entry) error {
	for _, entry := range entries {
		var schemas []activedirectory.AttributeSchema
		values := make(map[string][]string)

		// Map attributes to their schemas
		for _, attribute := range entry.Attributes {
			if schema, ok := adInstance.SchemaAttributeMap[attribute.Name]; ok {
				schemas = append(schemas, schema)
				values[attribute.Name] = attribute.Values
			}
		}

		// Print entry header
		fmt.Println(strings.Repeat("─", 80)) // Horizontal separator
		fmt.Printf("DN: %s\n", entry.DN)
		fmt.Println(strings.Repeat("─", 80)) // Horizontal separator

		// Iterate through schemas and print their details
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

			// Print attribute values using the schema's transformer
			attrValues, ok := values[schema.AttributeLDAPName]
			if !ok || len(attrValues) == 0 {
				fmt.Println("    └──Value: [No values]")
				continue
			}

			for j, rawValue := range attrValues {
				var transformedValue interface{}
				var err error

				// Use the schema's transformer, if available
				if schema.AttributeFieldType.TransformMethod != nil {
					transformedValue, err = schema.AttributeFieldType.TransformMethod.Transform([]string{rawValue})
					if err != nil {
						log.Printf("Error transforming attribute %s: %v\n", schema.AttributeLDAPName, err)
						continue
					}
				} else {
					// use the raw value if no transformer is defined
					log.Printf("No transformer was defined for %s\n", schema.AttributeLDAPName)
					transformedValue = rawValue
				}

				// Determine if it's the last value for formatting
				valuePrefix := "    ├──Value:"
				if j == len(attrValues)-1 {
					valuePrefix = "    └──Value:"
				}
				fmt.Printf("%s %v\n", valuePrefix, transformedValue)
			}
		}
	}

	return nil
}
