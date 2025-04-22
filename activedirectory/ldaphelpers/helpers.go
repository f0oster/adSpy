package ldaphelpers

import (
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

type notFilter struct {
	part Filter
}

func Not(f Filter) Filter {
	return notFilter{part: f}
}
func (f notFilter) String() string {
	return "(!" + f.part.String() + ")"
}

// Comparison operators
func Eq(attr, value string) Filter {
	return rawFilter("(" + attr + "=" + value + ")")
}
func Present(attr string) Filter {
	return rawFilter("(" + attr + "=*)")
}
