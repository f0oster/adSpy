package activedirectory

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"f0oster/adspy/activedirectory/formatters"

	"github.com/go-ldap/ldap/v3"
)

type ActiveDirectoryInstance struct {
	BaseDn               string
	DomainControllerFQDN string
	PageSize             uint32
	HighestCommittedUSN  int64
	SchemaMap            map[string]AttributeSchema
	ldapConnection       *ldap.Conn
}

func NewActiveDirectoryInstance(baseDn string, domainControllerDn string, pageSize uint32) *ActiveDirectoryInstance {
	return &ActiveDirectoryInstance{
		BaseDn:               baseDn,
		DomainControllerFQDN: domainControllerDn,
		PageSize:             pageSize,
		SchemaMap:            make(map[string]AttributeSchema),
	}
}

// Connect to the Active Directory Domain Controller
func (ad *ActiveDirectoryInstance) Connect(username, password string) bool {
	bindString := fmt.Sprintf("ldap://%s:389", ad.DomainControllerFQDN)
	var err error
	ad.ldapConnection, err = ldap.DialURL(bindString)
	if err != nil {
		log.Printf("Failed to connect to LDAP server: %v\n", err)
		return false
	}

	// TODO: LDAPS, IWA, etc
	err = ad.ldapConnection.Bind(username, password)
	if err != nil {
		ad.ldapConnection.Close()
		log.Printf("Failed to bind to LDAP server: %v\n", err)
		return false
	}

	return true
}

// Load schema data dynamically
func (ad *ActiveDirectoryInstance) LoadSchema() error {
	schemaBaseDN := "CN=Schema,CN=Configuration," + ad.BaseDn

	// Query for attribute schema
	attributesRequest := ldap.NewSearchRequest(
		schemaBaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0, 0, false,
		"(objectClass=attributeSchema)", // Searching for attributes
		[]string{"cn", "lDAPDisplayName", "attributeID", "attributeSyntax", "oMSyntax"},
		nil,
	)

	// Perform paged search for attributes
	attributesResults, err := ad.ldapConnection.SearchWithPaging(attributesRequest, ad.PageSize)
	if err != nil {
		return fmt.Errorf("failed to search for attributes: %v", err)
	}

	// Process the results and store schema data
	for _, entry := range attributesResults.Entries {
		attributeName := entry.GetAttributeValue("cn")
		ldapDisplayName := entry.GetAttributeValue("lDAPDisplayName")
		attributeID := entry.GetAttributeValue("attributeID")
		attributeSyntax := entry.GetAttributeValue("attributeSyntax")
		oMSyntax := entry.GetAttributeValue("oMSyntax")

		// goNativeType := determineGoNativeType(attributeSyntax, oMSyntax)
		goNativeType, err := mapSchemaToTypes(attributeSyntax, oMSyntax)

		if err != nil {
			return fmt.Errorf("error mapping schema to types: %v", err)
		}

		fmt.Println("Adding type for:", ldapDisplayName)

		ad.SchemaMap[ldapDisplayName] = AttributeSchema{
			AttributeName:      attributeName,
			AttributeLDAPName:  ldapDisplayName,
			AttributeID:        attributeID,
			AttributeSyntax:    attributeSyntax,
			AttributeOMSyntax:  oMSyntax,
			AttributeFieldType: *goNativeType,
		}
	}

	return nil
}

// fetch the highest committed USN from the target domain controller
func (ad *ActiveDirectoryInstance) FetchHighestUSN() error {
	highestCommittedUsnSearchRequest := ldap.NewSearchRequest(
		"", // Root DSE
		ldap.ScopeBaseObject,
		ldap.NeverDerefAliases,
		0, 0, false,
		"(objectClass=*)",
		[]string{"highestCommittedUSN"},
		nil,
	)

	highestCommittedUsnSearchResults, err := ad.ldapConnection.Search(highestCommittedUsnSearchRequest)
	if err != nil {
		return fmt.Errorf("failed to fetch highestCommittedUSN from Root DSE: %v", err)
	}

	// Check if the attribute was found
	if len(highestCommittedUsnSearchResults.Entries) > 0 {
		entry := highestCommittedUsnSearchResults.Entries[0].GetAttributeValue("highestCommittedUSN")
		ad.HighestCommittedUSN, err = strconv.ParseInt(entry, 10, 64)
		if err != nil {
			return fmt.Errorf("error converting highestCommittedUSN to int: %v", err)
		}
		fmt.Printf("Highest Committed USN: %d\n", ad.HighestCommittedUSN)
	} else {
		return fmt.Errorf("highestCommittedUSN not found in the Root DSE: %s", ad.BaseDn)
	}

	return nil
}

// unmarshalAttributeData unmarshals raw LDAP attribute values based on schema
// TODO: rewrite - should marshal to the native types - not transform for output
// TODO: transforming for output should be handled elsewhere
func (ad *ActiveDirectoryInstance) UnmarshalAttributeData(attributeLDAPName string, rawValue string) (interface{}, error) {
	attrSchema, exists := ad.SchemaMap[attributeLDAPName]
	if !exists {
		return nil, fmt.Errorf("attribute schema not found for: %v", attributeLDAPName)
	}

	switch attrSchema.AttributeFieldType.GoNativeType {
	case "string":
		return rawValue, nil
	case "int":
		parsedInt, err := strconv.Atoi(rawValue)
		if err != nil {
			return nil, fmt.Errorf("failed to parse integer: %v", err)
		}
		return parsedInt, nil
	case "bool":
		parsedBool, err := strconv.ParseBool(rawValue)
		if err != nil {
			return nil, fmt.Errorf("failed to parse boolean: %v", err)
		}
		return parsedBool, nil
	case "int64":
		parsedInt64, err := strconv.ParseInt(rawValue, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse large integer: %v", err)
		}
		return parsedInt64, nil
	case "time.Time":
		parsedTime, err := time.Parse("20060102150405.0Z", rawValue)
		if err != nil {
			return nil, fmt.Errorf("failed to parse time: %v", err)
		}
		return parsedTime, nil
	case "[]byte":
		if attributeLDAPName == "objectSid" {
			return formatters.ConvertSIDToString([]byte(rawValue))
		} else if strings.HasSuffix(strings.ToLower(attributeLDAPName), "guid") {
			return formatters.FormatADGuidAsString([]byte(rawValue)), nil
		} else {
			return []byte(rawValue), nil
		}
	default:
		return nil, fmt.Errorf("unsupported Go type (%s) for attribute %v", attrSchema.AttributeFieldType.GoNativeType, attributeLDAPName)
	}
}

// perform a paged LDAP query and callback per page
func (ad *ActiveDirectoryInstance) FetchPagedEntriesWithCallback(
	filter string, pageSize uint32, processPage func(adInstance *ActiveDirectoryInstance, entries []*ldap.Entry) error,
) error {
	pageControl := ldap.NewControlPaging(pageSize)
	pageRequest := ldap.NewSearchRequest(
		ad.BaseDn,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0, 0, false,
		filter,
		[]string{}, // Fetch all attributes
		[]ldap.Control{pageControl},
	)

	for {
		searchResults, err := ad.ldapConnection.Search(pageRequest)
		if err != nil {
			return fmt.Errorf("LDAP search failed: %w", err)
		}

		// Process the current page of entries
		if err := processPage(ad, searchResults.Entries); err != nil {
			return fmt.Errorf("processing page failed: %w", err)
		}

		// Check if there's a next page
		pagingControl := searchResults.Controls[0].(*ldap.ControlPaging)
		if pagingControl.Cookie == nil || len(pagingControl.Cookie) == 0 {
			break // No more pages
		}
		pageControl.SetCookie(pagingControl.Cookie)
	}

	return nil
}
