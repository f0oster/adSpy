package activedirectory

import (
	"fmt"
	"log"
	"strconv"

	"f0oster/adspy/activedirectory/schema"
	"f0oster/adspy/config"

	"github.com/go-ldap/ldap/v3"
)

// TODO: Separate related functionality into their own files
// TODO: Separate exported types (ie: PesistableADObject?) to a model package
func NewActiveDirectoryInstance(config config.ADSpyConfiguration) (*ActiveDirectoryInstance, error) {

	ad := &ActiveDirectoryInstance{
		BaseDn:               config.BaseDN,
		DomainControllerFQDN: config.DcFQDN,
		PageSize:             config.PageSize,
		SchemaRegistry:       schema.NewSchemaRegistry(),
	}

	ok := ad.connect(config.Username, config.Password)

	if !ok {
		return nil, fmt.Errorf("failed to connect to the Active Directory Domain")
	}

	err := ad.loadSchema()

	if err != nil {
		return nil, fmt.Errorf("failed to load schema: %w", err)
	}

	// Initialize parser after schema is loaded
	ad.parser = NewParser(ad.SchemaRegistry)

	err = ad.fetchDomainGUID()

	if err != nil {
		return nil, fmt.Errorf("failed to fetch DomainGUID: %w", err)
	}

	return ad, nil

}

// Connect to the Active Directory Domain Controller
func (ad *ActiveDirectoryInstance) connect(username, password string) bool {
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

// Load AttributeSchema data dynamically from the Schema partition
func (ad *ActiveDirectoryInstance) loadSchema() error {

	schemaBaseDN := "CN=Schema,CN=Configuration," + ad.BaseDn

	attributesRequest := ldap.NewSearchRequest(
		schemaBaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0, 0, false,
		"(objectClass=attributeSchema)", // Searching for attributeSchema
		[]string{"cn", "lDAPDisplayName", "attributeID", "attributeSyntax", "oMSyntax", "isSingleValued"},
		nil, // no control
	)

	// Perform paged search for attributes
	attributesResults, err := ad.ldapConnection.SearchWithPaging(attributesRequest, ad.PageSize)
	if err != nil {
		return fmt.Errorf("failed to search for attributes: %v", err)
	}

	// Process the objectAttributes and store schema data
	for _, entry := range attributesResults.Entries {
		attributeName := entry.GetAttributeValue("cn")
		ldapDisplayName := entry.GetAttributeValue("lDAPDisplayName")
		attributeID := entry.GetAttributeValue("attributeID")
		attributeSyntax := entry.GetAttributeValue("attributeSyntax")
		oMSyntax := entry.GetAttributeValue("oMSyntax")
		isSingleValued := entry.GetAttributeValue("isSingleValued")

		if isSingleValued != "TRUE" && isSingleValued != "FALSE" {
			return fmt.Errorf("invalid isSingleValued value: %q", isSingleValued)
		}
		singleValued := isSingleValued == "TRUE"

		attributeFieldType, err := ad.SchemaRegistry.Lookup(attributeSyntax, oMSyntax, ldapDisplayName)

		if err != nil {
			return fmt.Errorf("error mapping schema to types: %v", err)
		}

		schemaEntry := schema.AttributeSchema{
			AttributeName:           attributeName,
			AttributeLDAPName:       ldapDisplayName,
			AttributeID:             attributeID,
			AttributeSyntax:         attributeSyntax,
			AttributeOMSyntax:       oMSyntax,
			AttributeFieldType:      *attributeFieldType,
			AttributeIsSingleValued: singleValued,
		}
		// fmt.Printf("Adding type: %s\n", ldapDisplayName) // for debugging
		ad.SchemaRegistry.RegisterAttributeSchema(&schemaEntry)
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

func (ad *ActiveDirectoryInstance) fetchDomainGUID() error {
	domainGUIDSearchRequest := ldap.NewSearchRequest(
		ad.BaseDn,
		ldap.ScopeBaseObject,
		ldap.NeverDerefAliases,
		0, 0, false,
		"(objectClass=*)",
		[]string{"objectGUID"},
		nil,
	)

	domainGUIDSearchResults, err := ad.ldapConnection.Search(domainGUIDSearchRequest)
	if err != nil {
		return fmt.Errorf("failed to fetch domainGUID from Root DSE: %v", err)
	}

	adObject, err := ad.parser.parseEntry(domainGUIDSearchResults.Entries[0])
	if err != nil {
		return fmt.Errorf("failed to fetch domain DN object details from Root DSE: %v", err)
	}

	ad.DomainId = adObject.ObjectGUID
	fmt.Printf("Domain GUID: %s", adObject.ObjectGUID.String())

	return nil
}

// perform a paged LDAP query and callback per page
func (ad *ActiveDirectoryInstance) ForEachLDAPPage(
	filter string, pageSize uint32, pageHandlerCallback func(adInstance *ActiveDirectoryInstance, entries []*ldap.Entry) error,
) error {

	log.Println("LDAPFilter:", filter)

	// sdFlagsControl := ldaphelpers.CreateSDFlagsControl() // go-ldap added support for this control
	sdFlagsControl := ldap.NewControlMicrosoftSDFlags()
	pageControl := ldap.NewControlPaging(pageSize)
	showDeletedControl := ldap.NewControlMicrosoftShowDeleted()
	pageRequest := ldap.NewSearchRequest(
		ad.BaseDn,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0, 0, false,
		filter,
		// []string{"memberOf", "objectGUID", "userPrincipalName", "objectCategory"},
		[]string{}, // Fetch all attributes
		[]ldap.Control{pageControl, sdFlagsControl, showDeletedControl},
	)

	for {
		searchResults, err := ad.ldapConnection.Search(pageRequest)
		if err != nil {
			return fmt.Errorf("LDAP search failed: %w", err)
		}

		// Process the current page of entries
		if err := pageHandlerCallback(ad, searchResults.Entries); err != nil {
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


func (obj *ActiveDirectoryObject) GetNormalizedAttribute(attrName string) (string, bool) {
	attr, ok := obj.AttributeValues[attrName]
	if !ok || attr == nil || attr.NormalizedValue == nil {
		return "", false
	}
	val, err := attr.NormalizedValue.AsString()
	if err != nil {
		return "", false
	}
	return val, true
}


