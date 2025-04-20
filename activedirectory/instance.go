package activedirectory

import (
	"fmt"
	"log"
	"strconv"

	"f0oster/adspy/activedirectory/schema"

	"github.com/f0oster/gontsd"
	"github.com/go-ldap/ldap/v3"
	"github.com/google/uuid"
)

type ActiveDirectoryInstance struct {
	BaseDn               string
	DomainControllerFQDN string
	PageSize             uint32
	HighestCommittedUSN  int64
	SchemaRegistry       *schema.SchemaRegistry
	ldapConnection       *ldap.Conn
	DomainId             uuid.UUID
}

func NewActiveDirectoryInstance(baseDn string, domainControllerDn string, pageSize uint32) *ActiveDirectoryInstance {
	return &ActiveDirectoryInstance{
		BaseDn:               baseDn,
		DomainControllerFQDN: domainControllerDn,
		PageSize:             pageSize,
		SchemaRegistry:       schema.NewSchemaRegistry(),
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
		fmt.Printf("Adding type: %s\n", ldapDisplayName)
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

// create the LDAP_SERVER_SD_FLAGS_OID extended control to return ntSecurityDescriptor
func CreateSDFlagsControl() ldap.Control {
	// Construct the BER-encoded sequence for the SD flags
	// [0x30 0x03 0x02 0x01 0x07] for SD flags
	// https://learn.microsoft.com/en-us/previous-versions/windows/desktop/ldap/ldap-server-sd-flags-oid
	// https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-adts/3888c2b7-35b9-45b7-afeb-b772aa932dd0
	// https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-adts/3c5e87db-4728-4f29-b164-01dd7d7391ea
	// value := []byte{0x30, 0x03, 0x02, 0x01, 0x07}

	return ldap.NewControlString("1.2.840.113556.1.4.801", true, fmt.Sprintf("%c%c%c%c%c", 48, 3, 2, 1, 7))
}

// perform a paged LDAP query and callback per page
func (ad *ActiveDirectoryInstance) FetchPagedEntriesWithCallback(
	filter string, pageSize uint32, processPage func(adInstance *ActiveDirectoryInstance, entries []*ldap.Entry) error,
) error {

	log.Println("LDAPFilter:", filter)

	sdFlagsControl := CreateSDFlagsControl()
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

type ActiveDirectoryObject struct {
	DN                   string
	PrimaryObjectClass   string
	NTSecurityDescriptor *gontsd.SecurityDescriptor
	AttributeValues      map[string]*schema.AttributeValue
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

func (obj *ActiveDirectoryObject) GetObjectGUID() (uuid.UUID, error) {
	rawValue, ok := obj.AttributeValues["objectGUID"]
	if !ok {
		return uuid.Nil, nil // attribute not present
	}

	switch v := rawValue.InterpretedValue.First().(type) {
	case []uuid.UUID:
		if len(v) == 0 {
			return uuid.Nil, fmt.Errorf("objectGUID is an empty []uuid.UUID slice")
		}
		return v[0], nil
	case []interface{}:
		if len(v) == 0 {
			return uuid.Nil, fmt.Errorf("objectGUID is an empty []interface{} slice")
		}
		guid, ok := v[0].(uuid.UUID)
		if !ok {
			return uuid.Nil, fmt.Errorf("objectGUID[0] is not a uuid.UUID (got %T)", v[0])
		}
		return guid, nil
	default:
		return uuid.Nil, fmt.Errorf("objectGUID is of unsupported type: %T", rawValue)
	}
}

type NormalizedADObject struct {
	DN          string              `json:"dn"`
	ObjectClass string              `json:"object_class"`
	Attributes  map[string][]string `json:"attributes"`
}

func ToNormalizedADObject(obj ActiveDirectoryObject) NormalizedADObject {
	flat := NormalizedADObject{
		DN:          obj.DN,
		ObjectClass: obj.PrimaryObjectClass,
		Attributes:  make(map[string][]string),
	}

	for name, attr := range obj.AttributeValues {
		str := attr.NormalizedValue.Values
		flat.Attributes[name] = str
	}

	return flat
}

func (ad *ActiveDirectoryInstance) parseAttribute(attr *ldap.EntryAttribute, schemaAttr schema.AttributeSchema) (*schema.AttributeValue, error) {
	byteValues := attr.ByteValues

	if schemaAttr.AttributeFieldType.Normalizer == nil {
		return nil, fmt.Errorf("normalizer not defined for attribute %s", schemaAttr.AttributeLDAPName)
	}

	// Normalize
	normalizedStrings, err := schemaAttr.AttributeFieldType.Normalizer.Normalize(byteValues)
	if err != nil {
		log.Printf("Failed to normalize %s: %v", attr.Name, err)
	}

	/*
		log.Printf("%s %v (%s / %s / %s / SingleValued: %v): %s",
			attr.Name,
			normalizedStrings,
			schemaAttr.AttributeFieldType.SyntaxName,
			schemaAttr.AttributeSyntax,
			schemaAttr.AttributeOMSyntax,
			schemaAttr.AttributeIsSingleValued,
			schemaAttr.AttributeFieldType.GoType.String(),
		)
	*/

	normalized := &schema.NormalizedValue{Values: normalizedStrings}

	// Interpret
	interpreted := &schema.InterpretedValue{}
	interpretedVal, err := schemaAttr.AttributeFieldType.Interpreter.Interpret(byteValues)
	if err != nil {
		log.Printf("error interpreting %s -  %v\n", attr.Name, err)
	} else {
		switch v := interpretedVal.(type) {
		case []interface{}:
			interpreted.Values = append(interpreted.Values, v...)
		default:
			interpreted.Values = append(interpreted.Values, v)
		}
	}

	return &schema.AttributeValue{
		Name:             attr.Name,
		NormalizedValue:  normalized,
		InterpretedValue: interpreted,
		LDAPRawValue:     attr.Values,
		LDAPByteValue:    attr.ByteValues,
		Schema:           &schemaAttr,
	}, nil
}

func (ad *ActiveDirectoryInstance) ParseLDAPAttributeValues(entry *ldap.Entry) (ActiveDirectoryObject, error) {
	objectAttributes := make(map[string]*schema.AttributeValue)
	var (
		nTSecurityDescriptor *gontsd.SecurityDescriptor
		primaryObjectClass   string
	)

	for _, attr := range entry.Attributes {

		// schemaAttr, ok := ad.SchemaAttributeMap[attr.Name]
		schemaAttr, ok := ad.SchemaRegistry.GetAttributeSchema(attr.Name)
		if !ok {
			log.Printf("Unknown attribute parsed: %s\n", attr.Name)
			continue
		}

		parsedAttr, err := ad.parseAttribute(attr, *schemaAttr)
		if err != nil {
			return ActiveDirectoryObject{}, fmt.Errorf("failed to parse attribute %s: %w", attr.Name, err)
		}
		if parsedAttr == nil {
			continue
		}

		if attr.Name == "objectClass" && parsedAttr.NormalizedValue != nil {
			if len(parsedAttr.NormalizedValue.Values) > 0 {
				primaryObjectClass = parsedAttr.NormalizedValue.Values[len(parsedAttr.NormalizedValue.Values)-1]
			}
		}

		if attr.Name == "nTSecurityDescriptor" {
			if len(attr.ByteValues) > 0 {
				sd, err := gontsd.Parse(parsedAttr.LDAPByteValue[0])
				if err != nil {
					fmt.Printf("failed to parse nTSecurityDescriptor for DN %s: %v\n", entry.DN, err)
				} else {
					nTSecurityDescriptor = sd
				}
			}
		}

		objectAttributes[attr.Name] = parsedAttr
	}

	return ActiveDirectoryObject{
		DN:                   entry.DN,
		PrimaryObjectClass:   primaryObjectClass,
		NTSecurityDescriptor: nTSecurityDescriptor,
		AttributeValues:      objectAttributes,
	}, nil
}

func (ad *ActiveDirectoryInstance) FetchEntries(filter string) ([]ActiveDirectoryObject, error) {
	sdFlagsControl := CreateSDFlagsControl()

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
		adObjects = append(adObjects, adObject)
	}

	return adObjects, nil
}
