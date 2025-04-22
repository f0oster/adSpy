package transformers

import (
	"encoding/base64"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/f0oster/gontsd"
	"github.com/google/uuid"
)

type Normalizer interface {
	Normalize(values [][]byte) ([]string, error) // always string
}

type Interpreter interface {
	Interpret(values [][]byte) (interface{}, error) // native Go types
}

type SimpleStringFormatter struct{}

func (t SimpleStringFormatter) Normalize(values [][]byte) ([]string, error) {
	return t.transform(values)
}

func (t SimpleStringFormatter) Interpret(values [][]byte) (interface{}, error) {
	return t.transform(values)
}

func (t SimpleStringFormatter) transform(values [][]byte) ([]string, error) {
	result := make([]string, len(values))
	for i, b := range values {
		if !utf8.Valid(b) {
			fmt.Println("warning: [SimpleStringFormatter] transform performed base64 encode - data was a binary blob and not a valid utf8 string")
			result[i] = base64.StdEncoding.EncodeToString(b)
		} else {
			result[i] = string(b)
		}
	}
	return result, nil
}

type SIDFormatter struct{}

func (t SIDFormatter) Normalize(values [][]byte) ([]string, error) {
	return t.transform(values)
}

func (t SIDFormatter) Interpret(values [][]byte) (interface{}, error) {
	return t.transform(values)
}

func (t SIDFormatter) transform(values [][]byte) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}

	sids := make([]string, 0, len(values))
	for _, sidBytes := range values {
		sid, err := ConvertSIDToString(sidBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse SID: %w", err)
		}
		sids = append(sids, sid)
	}

	return sids, nil
}

type ADGuidFormatter struct{}

// adGuidToRFC4122Uuid converts a slice of Active Directory GUIDs (little-endian format)
// into RFC4122-compliant uuid.UUID instances.
func adGuidToRFC4122Uuid(values [][]byte) ([]uuid.UUID, error) {
	if len(values) == 0 {
		return nil, nil
	}

	result := make([]uuid.UUID, 0, len(values))
	for i, adGuid := range values {
		if len(adGuid) != 16 {
			return nil, fmt.Errorf("invalid GUID at index %d: expected 16 bytes, got %d", i, len(adGuid))
		}

		rfcBytes := make([]byte, 16)
		copy(rfcBytes, adGuid)

		rfcBytes[0], rfcBytes[1], rfcBytes[2], rfcBytes[3] = rfcBytes[3], rfcBytes[2], rfcBytes[1], rfcBytes[0]
		rfcBytes[4], rfcBytes[5] = rfcBytes[5], rfcBytes[4]
		rfcBytes[6], rfcBytes[7] = rfcBytes[7], rfcBytes[6]

		u, err := uuid.FromBytes(rfcBytes)
		if err != nil {
			return nil, fmt.Errorf("invalid UUID generated from AD GUID at index %d: %w", i, err)
		}

		result = append(result, u)
	}

	return result, nil
}

func (t ADGuidFormatter) Normalize(values [][]byte) ([]string, error) {
	uuids, err := adGuidToRFC4122Uuid(values)
	if err != nil {
		return nil, err
	}

	result := make([]string, len(uuids))
	for i, u := range uuids {
		result[i] = u.String()
	}
	return result, nil
}

func (t ADGuidFormatter) Interpret(values [][]byte) (interface{}, error) {
	uuids, err := adGuidToRFC4122Uuid(values)
	if err != nil {
		return nil, err
	}

	result := make([]interface{}, len(uuids))
	for i, u := range uuids {
		result[i] = u
	}
	return result, nil
}

type ADFiletimeFormatter struct{}

const (
	filetimeEpochOffset = 116444736000000000
	filetimeNever       = int64(9223372036854775807)
)

func fromFileDateTime(values [][]byte) ([]*time.Time, error) {

	if len(values) == 0 {
		return nil, nil
	}

	times := make([]*time.Time, len(values))

	for i, b := range values {
		str := string(b)
		if str == "" || str == "0" {
			times[i] = nil
			continue
		}

		ftVal, err := parseInt64(str)
		if err != nil {
			return nil, fmt.Errorf("invalid FILETIME integer: %w", err)
		}

		if ftVal == 0 || ftVal == filetimeNever {
			times[i] = nil
			continue
		}

		nsSinceUnix := (ftVal - filetimeEpochOffset) * 100
		t := time.Unix(0, nsSinceUnix).UTC()
		times[i] = &t
	}
	return times, nil
}

func (t ADFiletimeFormatter) Interpret(values [][]byte) (interface{}, error) {

	times, err := fromFileDateTime(values)
	if err != nil {
		return nil, fmt.Errorf("failed to interpret filetime integer: %w", err)
	}

	return times, nil
}

func (t ADFiletimeFormatter) Normalize(values [][]byte) ([]string, error) {

	times, err := fromFileDateTime(values)
	if err != nil {
		return nil, fmt.Errorf("failed to interpret filetime integer: %w", err)
	}

	timeStrings := make([]string, len(values))
	for i, time := range times {
		if time == nil {
			timeStrings[i] = "N/A"
			continue
		}
		timeStrings[i] = time.String()
	}

	return timeStrings, nil
}

type NTSecurityDescriptorFormatter struct{}

func (t NTSecurityDescriptorFormatter) Interpret(values [][]byte) (interface{}, error) {

	ntSecurityDescriptor, err := gontsd.Parse(values[0])

	if err != nil {
		return nil, fmt.Errorf("failed to interpret nTSecurityDescriptor: %w", err)
	}

	return ntSecurityDescriptor, nil
}

func (t NTSecurityDescriptorFormatter) Normalize(values [][]byte) ([]string, error) {

	b64EncodednTSecurityDescriptor := make([]string, len(values))
	for i, b := range values {
		if utf8.Valid(b) {
			return nil, fmt.Errorf("ntSecurityDescriptor field should not contain a valid utf8 string")
		}
		b64EncodednTSecurityDescriptor[i] = base64.StdEncoding.EncodeToString(b)
	}

	// return as []string
	return b64EncodednTSecurityDescriptor, nil
}

type LDAPTimeFormatter struct {
	Layout string
}

func (t LDAPTimeFormatter) parseLDAPTime(values [][]byte) ([]time.Time, error) {
	times := make([]time.Time, len(values))
	for i, b := range values {
		s := string(b)
		ts, err := time.Parse(t.Layout, s)
		if err != nil {
			return nil, err
		}
		times[i] = ts
	}

	return times, nil
}

func (t LDAPTimeFormatter) Normalize(values [][]byte) ([]string, error) {
	times, err := t.parseLDAPTime(values)
	if err != nil {
		return nil, fmt.Errorf("failed to parse LDAP times: %w", err)
	}

	strTimes := make([]string, len(times))
	for i, tm := range times {
		strTimes[i] = tm.String()
	}

	return strTimes, nil
}

type Base64Formatter struct {
	Layout string
}

func (t Base64Formatter) Normalize(values [][]byte) ([]string, error) {
	sanitized := make([]string, len(values))
	for i, b := range values {
		sanitized[i] = base64.StdEncoding.EncodeToString(b)
	}
	return sanitized, nil
}
