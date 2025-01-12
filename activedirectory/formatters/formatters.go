package formatters

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// convertSIDToString formats a byte array containing an object SID to a string in SID format.
func ConvertSIDToString(sidBytes []byte) (string, error) {
	// Minimum SID length is 8 bytes: revision (1), sub-authority count (1), authority (6)
	if len(sidBytes) < 8 {
		return "", fmt.Errorf("invalid SID: too short")
	}

	// Read the revision (1 byte)
	revision := sidBytes[0]

	// Read the sub-authority count (1 byte)
	subAuthorityCount := int(sidBytes[1])

	// Read the authority (6 bytes, big-endian integer)
	authority := binary.BigEndian.Uint64(append([]byte{0, 0}, sidBytes[2:8]...))

	// Validate SID length for the sub-authorities
	expectedLength := 8 + (subAuthorityCount * 4)
	if len(sidBytes) < expectedLength {
		return "", fmt.Errorf("invalid SID: insufficient length for sub-authorities")
	}

	// Read the sub-authorities (4 bytes each, little-endian integers)
	var subAuthorities []uint32
	offset := 8
	for i := 0; i < subAuthorityCount; i++ {
		subAuthority := binary.LittleEndian.Uint32(sidBytes[offset : offset+4])
		subAuthorities = append(subAuthorities, subAuthority)
		offset += 4
	}

	// Format the SID string
	var sidBuffer bytes.Buffer
	sidBuffer.WriteString(fmt.Sprintf("S-%d-%d", revision, authority))
	for _, subAuthority := range subAuthorities {
		sidBuffer.WriteString(fmt.Sprintf("-%d", subAuthority))
	}

	return sidBuffer.String(), nil
}

// formatADGuidAsString formats a byte array containing an an AD GUID to an Active Directory GUID string.
func FormatADGuidAsString(rawValue []byte) string {
	return fmt.Sprintf("%02x%02x%02x%02x-%02x%02x-%02x%02x-%02x%02x-%02x%02x%02x%02x%02x%02x",
		rawValue[0], rawValue[1], rawValue[2], rawValue[3],
		rawValue[4], rawValue[5], rawValue[6], rawValue[7],
		rawValue[8], rawValue[9], rawValue[10], rawValue[11],
		rawValue[12], rawValue[13], rawValue[14], rawValue[15],
	)
}
