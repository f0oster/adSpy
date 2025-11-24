package ldaphelpers

import (
	"fmt"
	"strings"
)


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

func Eq(attr, value string) Filter {
	return rawFilter("(" + attr + "=" + value + ")")
}
