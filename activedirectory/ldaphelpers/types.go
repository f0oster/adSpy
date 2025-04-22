package ldaphelpers

const (
	AllObjects      = "(objectClass=*)"
	AllGroupObjects = "(objectClass=group)"
	AllUserObjects  = "(&(objectCategory=person)(objectClass=user))"
)
