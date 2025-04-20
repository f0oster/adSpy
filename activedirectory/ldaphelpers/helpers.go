package ldaphelpers

import (
	"f0oster/adspy/activedirectory"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/go-ldap/ldap/v3"
)

const (
	AllObjects      = "(objectClass=*)"
	AllGroupObjects = "(objectClass=group)"
	AllUserObjects  = "(&(objectCategory=person)(objectClass=user))"
)

type Filter interface {
	String() string
}

type rawFilter string

func (f rawFilter) String() string {
	return string(f)
}

// Logical operators
type andFilter struct {
	parts []Filter
}

func And(filters ...Filter) Filter {
	return andFilter{parts: filters}
}
func (f andFilter) String() string {
	var parts []string
	for _, p := range f.parts {
		parts = append(parts, p.String())
	}
	return "(&" + strings.Join(parts, "") + ")"
}

type orFilter struct {
	parts []Filter
}

func Or(filters ...Filter) Filter {
	return orFilter{parts: filters}
}
func (f orFilter) String() string {
	var parts []string
	for _, p := range f.parts {
		parts = append(parts, p.String())
	}
	return "(|" + strings.Join(parts, "") + ")"
}

type notFilter struct {
	part Filter
}

func Not(f Filter) Filter {
	return notFilter{part: f}
}
func (f notFilter) String() string {
	return "(!" + f.part.String() + ")"
}

// Comparison operators
func Eq(attr, value string) Filter {
	return rawFilter("(" + attr + "=" + value + ")")
}
func Present(attr string) Filter {
	return rawFilter("(" + attr + "=*)")
}

func PrintToConsole(adInstance *activedirectory.ActiveDirectoryInstance, entries []*ldap.Entry) error {
	for _, entry := range entries {
		adObject, err := adInstance.ParseLDAPAttributeValues(entry)
		if err != nil {
			log.Printf("Skipping entry for DN %s: %v", entry.DN, err)
			continue
		}

		fmt.Println(strings.Repeat("─", 80))
		fmt.Printf("DN: %s\n", adObject.DN)
		fmt.Println(strings.Repeat("─", 80))

		sortedAttrNames := make([]string, 0, len(adObject.AttributeValues))
		for name := range adObject.AttributeValues {
			sortedAttrNames = append(sortedAttrNames, name)
		}
		sort.Strings(sortedAttrNames)

		for i, attrName := range sortedAttrNames {
			attr := adObject.AttributeValues[attrName]
			prefix := "├───"
			if i == len(sortedAttrNames)-1 {
				prefix = "└───"
			}

			schema := attr.Schema
			fmt.Printf("%sAttribute: %s (%s)\n", prefix, schema.AttributeName, schema.AttributeLDAPName)
			fmt.Printf("    ├──AttributeID: %s\n", schema.AttributeID)
			fmt.Printf("    ├──AttributeSyntax: %s\n", schema.AttributeSyntax)
			fmt.Printf("    ├──AttributeOMSyntax: %s\n", schema.AttributeOMSyntax)
			fmt.Printf("    ├──SchemaSyntax: %s\n", schema.AttributeFieldType.SyntaxName)
			fmt.Printf("    ├──GoType: %s\n", schema.AttributeFieldType.GoType.String())

			// Display the values
			asStr, err := attr.AsStringSlice()
			if err != nil {
				log.Printf("[error: %v]", err)
			}

			if len(asStr) == 0 {
				fmt.Println("    └──Value: [No values]")
				continue
			}

			for j, val := range asStr {
				valuePrefix := "    ├──Value:"
				if j == len(asStr)-1 {
					valuePrefix = "    └──Value:"
				}

				fmt.Printf("%s %v\n", valuePrefix, val)
			}
		}
	}
	return nil
}
