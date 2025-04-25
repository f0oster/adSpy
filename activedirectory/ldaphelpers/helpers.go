package ldaphelpers

import (
	"fmt"
	"strings"

	"github.com/go-ldap/ldap/v3"
)

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

// TODO: Organize / refactor
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

type geFilter struct {
	attr  string
	value int64
}

func (f geFilter) String() string {
	return fmt.Sprintf("(%s>=%d)", f.attr, f.value)
	// "(" + f.attr + ">=" + f.value + ")"
}

func Ge(attr string, value int64) Filter {
	return geFilter{attr: attr, value: value}
}

func Not(f Filter) Filter {
	return notFilter{part: f}
}
func (f notFilter) String() string {
	return "(!" + f.part.String() + ")"
}

func Eq(attr, value string) Filter {
	return rawFilter("(" + attr + "=" + value + ")")
}
func Present(attr string) Filter {
	return rawFilter("(" + attr + "=*)")
}
