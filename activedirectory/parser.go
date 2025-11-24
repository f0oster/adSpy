package activedirectory

import (
	"f0oster/adspy/activedirectory/ldaphelpers"
	"f0oster/adspy/activedirectory/schema"
	"f0oster/adspy/activedirectory/schema/accessors"
	"fmt"
	"log"

	"github.com/f0oster/gontsd"
	"github.com/go-ldap/ldap/v3"
	"github.com/google/uuid"
)

// Parser handles conversion of LDAP entries to ActiveDirectoryObjects.
type Parser struct {
	schemaRegistry *schema.SchemaRegistry
}

func NewParser(schemaRegistry *schema.SchemaRegistry) *Parser {
	return &Parser{
		schemaRegistry: schemaRegistry,
	}
}

// ParseResult represents the result of parsing a single LDAP entry.
// It contains either a successfully parsed object or an error.
type ParseResult struct {
	Object *ActiveDirectoryObject
	DN     string // Always populated for error reporting
	Error  error
}

// ParseEntries processes multiple LDAP entries and returns results for each.
// All entries get a ParseResult - either with Object or Error populated.
func (p *Parser) ParseEntries(entries []*ldap.Entry) []*ParseResult {
	results := make([]*ParseResult, 0, len(entries))

	for _, entry := range entries {
		obj, err := p.parseEntry(entry)
		result := &ParseResult{
			DN: entry.DN,
		}

		if err != nil {
			result.Error = err
		} else {
			result.Object = obj
		}

		results = append(results, result)
	}

	return results
}

// parseEntry converts a single LDAP entry to an ActiveDirectoryObject.
func (p *Parser) parseEntry(entry *ldap.Entry) (*ActiveDirectoryObject, error) {
	objectAttributes := make(map[string]*schema.AttributeValue)
	var (
		objectGUID           uuid.UUID
		primaryObjectClass   string
		nTSecurityDescriptor *gontsd.SecurityDescriptor
	)

	for _, attr := range entry.Attributes {
		attributeSchema, ok := p.schemaRegistry.GetAttributeSchema(attr.Name)
		if !ok {
			log.Printf("Skipping parsing for unknown attribute: %s\n", attr.Name)
			continue
		}

		parsedAttr, err := ldaphelpers.ParseAttribute(attr, *attributeSchema)
		if err != nil {
			return nil, fmt.Errorf("failed to parse attribute %s: %w", attr.Name, err)
		}
		if parsedAttr == nil {
			continue
		}

		// Extract primary object class
		if attr.Name == "objectClass" && parsedAttr.NormalizedValue != nil {
			if len(parsedAttr.NormalizedValue.Values) > 0 {
				primaryObjectClass, err = parsedAttr.NormalizedValue.LastStringInSlice()
				if err != nil {
					return nil, fmt.Errorf("failed to fetch the primary objectClass: %w", err)
				}
			}
		}

		if attr.Name == "objectGUID" {
			objectGUID, err = accessors.FirstAs[uuid.UUID](*parsedAttr.InterpretedValue)
			if err != nil {
				return nil, fmt.Errorf("type assertion failed: expected uuid.UUID, got %T", err)
			}
		}

		if attr.Name == "nTSecurityDescriptor" {
			if len(attr.ByteValues) > 0 {
				nTSecurityDescriptor, err = gontsd.Parse(parsedAttr.LDAPByteValue[0])
				if err != nil {
					// security descriptor parsing is not critical at this stage as the parser is incomplete
					log.Printf("failed to parse nTSecurityDescriptor for DN %s: %v\n", entry.DN, err)
				}
			}
		}

		objectAttributes[attr.Name] = parsedAttr
	}

	return &ActiveDirectoryObject{
		DN:                   entry.DN,
		ObjectGUID:           objectGUID,
		PrimaryObjectClass:   primaryObjectClass,
		NTSecurityDescriptor: nTSecurityDescriptor,
		AttributeValues:      objectAttributes,
	}, nil
}
