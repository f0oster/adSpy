package activedirectory

import (
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

type ActiveDirectoryObject struct {
	DN                   string
	ObjectGUID           uuid.UUID
	PrimaryObjectClass   string
	NTSecurityDescriptor *gontsd.SecurityDescriptor
	AttributeValues      map[string]*schema.AttributeValue
}

type ADSnapshot struct {
	DN          string              `json:"dn"`
	ObjectGUID  string              `json:"object_guid"`
	ObjectClass string              `json:"object_class"`
	Attributes  map[string][]string `json:"attributes"`
}
