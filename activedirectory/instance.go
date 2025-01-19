package activedirectory

import (
	"fmt"
	"log"
	"strconv"

	"github.com/go-ldap/ldap/v3"
	"github.com/google/uuid"
)

type ActiveDirectoryInstance struct {
	BaseDn               string
	DomainControllerFQDN string
	PageSize             uint32
	HighestCommittedUSN  int64
	SchemaAttributeMap   map[string]AttributeSchema
	ldapConnection       *ldap.Conn
	DomainId             uuid.UUID
}

func NewActiveDirectoryInstance(baseDn string, domainControllerDn string, pageSize uint32) *ActiveDirectoryInstance {
	return &ActiveDirectoryInstance{
		BaseDn:               baseDn,
		DomainControllerFQDN: domainControllerDn,
		PageSize:             pageSize,
		SchemaAttributeMap:   make(map[string]AttributeSchema),
	}
}

// Connect to the Active Directory Domain Controller
func (ad *ActiveDirectoryInstance) Connect(username, password string) bool {
	var err error

	bindString := fmt.Sprintf("ldap://%s:389", ad.DomainControllerFQDN)
	ad.ldapConnection, err = ldap.DialURL(bindString)
	if err != nil {
		log.Printf("Failed to connect to LDAP server: %v\n", err)
		return false
	}

	// TODO: LDAPS, IWA/GSSAPI, etc
	err = ad.ldapConnection.Bind(username, password)
	if err != nil {
		ad.ldapConnection.Close()
		log.Printf("Failed to bind to LDAP server: %v\n", err)
		return false
	}

	res, err := ad.ldapConnection.WhoAmI(nil)
	if err != nil {
		log.Printf("Failed to call WhoAmI(): %s\n", err)
		return false
	}
	fmt.Printf("Authenticated to %s as %s\n", bindString, res.AuthzID)

	return true
}

// Load AttributeSchema data dynamically
func (ad *ActiveDirectoryInstance) LoadSchema() error {
	schemaBaseDN := "CN=Schema,CN=Configuration," + ad.BaseDn

	attributesRequest := ldap.NewSearchRequest(
		schemaBaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0, 0, false,
		"(objectClass=attributeSchema)", // Searching for attributeSchema
		[]string{"cn", "lDAPDisplayName", "attributeID", "attributeSyntax", "oMSyntax"},
		nil, // no control
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

		goNativeType, err := mapSchemaToTypes(attributeSyntax, oMSyntax)

		if err != nil {
			return fmt.Errorf("error mapping schema to types: %v", err)
		}

		// fmt.Println("Adding type for:", ldapDisplayName)

		ad.SchemaAttributeMap[ldapDisplayName] = AttributeSchema{
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

// create the LDAP_SERVER_SD_FLAGS_OID extended control to return ntSecurityDescriptor
func CreateSDFlagsControl() ldap.Control {
	// Construct the BER-encoded sequence for the SD flags
	// [0x30 0x03 0x02 0x01 0x07] for SD flags = 7 (0x07)
	// https://learn.microsoft.com/en-us/previous-versions/windows/desktop/ldap/ldap-server-sd-flags-oid
	// https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-adts/3888c2b7-35b9-45b7-afeb-b772aa932dd0
	// https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-adts/3c5e87db-4728-4f29-b164-01dd7d7391ea
	value := []byte{0x30, 0x03, 0x02, 0x01, 0x07}

	return ldap.NewControlString("1.2.840.113556.1.4.801", true, string(value))
}

// perform a paged LDAP query and callback per page
func (ad *ActiveDirectoryInstance) FetchPagedEntriesWithCallback(
	filter string, pageSize uint32, processPage func(adInstance *ActiveDirectoryInstance, entries []*ldap.Entry) error,
) error {

	sdFlagsControl := CreateSDFlagsControl()

	pageControl := ldap.NewControlPaging(pageSize)
	pageRequest := ldap.NewSearchRequest(
		ad.BaseDn,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0, 0, false,
		filter,
		[]string{}, // Fetch all attributes
		[]ldap.Control{pageControl, sdFlagsControl},
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
