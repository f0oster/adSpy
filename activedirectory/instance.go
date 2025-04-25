package activedirectory

import (
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"

	"f0oster/adspy/activedirectory/ldaphelpers"
	"f0oster/adspy/activedirectory/schema"
	"f0oster/adspy/activedirectory/schema/accessors"
	"f0oster/adspy/config"

	"github.com/f0oster/gontsd"
	"github.com/go-ldap/ldap/v3"
	"github.com/google/uuid"
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

	adObject, err := ad.ParseLDAPAttributeValues(domainGUIDSearchResults.Entries[0])
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

	sdFlagsControl := ldaphelpers.CreateSDFlagsControl()
	pageControl := ldap.NewControlPaging(pageSize)
	pageRequest := ldap.NewSearchRequest(
		ad.BaseDn,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0, 0, false,
		filter,
		// []string{"memberOf", "objectGUID", "userPrincipalName", "objectCategory"},
		[]string{}, // Fetch all attributes
		[]ldap.Control{pageControl, sdFlagsControl},
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

func PrepareADSnapshot(adObj *ActiveDirectoryObject) *ADSnapshot {
	n := &ADSnapshot{
		DN:          adObj.DN,
		ObjectGUID:  adObj.ObjectGUID.String(),
		ObjectClass: adObj.PrimaryObjectClass,
		Attributes:  make(map[string][]string),
	}

	for k, v := range adObj.AttributeValues {
		n.Attributes[k] = v.NormalizedValue.Values // always []string
	}

	return n
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

func (obj *ActiveDirectoryObject) GetInterpretedAttribute(attrName string) (interface{}, bool) {
	attr, ok := obj.AttributeValues[attrName]
	if !ok || attr == nil || attr.NormalizedValue == nil {
		return "", false
	}

	return attr.InterpretedValue.Values, true
}

func ToADSnapshot(obj ActiveDirectoryObject) ADSnapshot {
	flat := ADSnapshot{
		DN:          obj.DN,
		ObjectGUID:  obj.ObjectGUID.String(),
		ObjectClass: obj.PrimaryObjectClass,
		Attributes:  make(map[string][]string),
	}

	for name, attr := range obj.AttributeValues {
		str := attr.NormalizedValue.Values
		flat.Attributes[name] = str
	}

	return flat
}

func (ad *ActiveDirectoryInstance) ParseLDAPAttributeValues(entry *ldap.Entry) (*ActiveDirectoryObject, error) {
	objectAttributes := make(map[string]*schema.AttributeValue)
	var (
		objectGUID           uuid.UUID
		primaryObjectClass   string
		nTSecurityDescriptor *gontsd.SecurityDescriptor
	)

	for _, attr := range entry.Attributes {

		attributeSchema, ok := ad.SchemaRegistry.GetAttributeSchema(attr.Name)
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
					// debug logging
					fmt.Printf("failed to parse nTSecurityDescriptor for DN %s: %v\n", entry.DN, err)
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

func (ad *ActiveDirectoryInstance) FetchEntries(filter string) ([]ActiveDirectoryObject, error) {
	sdFlagsControl := ldaphelpers.CreateSDFlagsControl()

	ldapSearch := ldap.NewSearchRequest(
		ad.BaseDn,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0, 0, false,
		filter,
		[]string{}, // Fetch all attributes
		[]ldap.Control{sdFlagsControl},
	)

	searchResults, err := ad.ldapConnection.Search(ldapSearch)
	if err != nil {
		return nil, fmt.Errorf("LDAP search failed: %w", err)
	}

	adObjects := make([]ActiveDirectoryObject, 0, len(searchResults.Entries))
	for _, entry := range searchResults.Entries {
		adObject, err := ad.ParseLDAPAttributeValues(entry)
		if err != nil {
			return nil, fmt.Errorf("failed to parse LDAP entry %q: %w", entry.DN, err)
		}
		adObjects = append(adObjects, *adObject)
	}

	return adObjects, nil
}

func PrintToConsole(adInstance *ActiveDirectoryInstance, entries []*ldap.Entry) error {
	// TODO: refactor to be more generic for generalized debugging?
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
