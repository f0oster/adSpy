package sddiff

import (
	"fmt"
	"strings"

	"github.com/f0oster/gontsd"
	"github.com/f0oster/gontsd/resolve"
)

// SDDiff represents the computed difference between two security descriptors.
type SDDiff struct {
	OwnerChanged        bool        `json:"owner_changed"`
	OldOwner            *SIDInfo    `json:"old_owner,omitempty"`
	NewOwner            *SIDInfo    `json:"new_owner,omitempty"`
	GroupChanged        bool        `json:"group_changed"`
	OldGroup            *SIDInfo    `json:"old_group,omitempty"`
	NewGroup            *SIDInfo    `json:"new_group,omitempty"`
	ControlFlagsChanged bool        `json:"control_flags_changed"`
	OldControlFlags     uint16      `json:"old_control_flags,omitempty"`
	NewControlFlags     uint16      `json:"new_control_flags,omitempty"`
	DACLDiff   *ACLDiffDTO `json:"dacl_diff,omitempty"`
	HasChanges bool        `json:"has_changes"`
}

// SIDInfo represents a security identifier with human-readable info.
type SIDInfo struct {
	Raw          string `json:"raw"`
	ResolvedName string `json:"resolved_name,omitempty"`
}

// ACLDiffDTO represents differences in an access control list.
type ACLDiffDTO struct {
	RevisionChanged bool         `json:"revision_changed"`
	OldRevision     uint8        `json:"old_revision,omitempty"`
	NewRevision     uint8        `json:"new_revision,omitempty"`
	ACEDiffs        []ACEDiffDTO `json:"ace_diffs"`
	// Full ACE lists for before/after view
	OldACEs []ACEStateDTO `json:"old_aces,omitempty"`
	NewACEs []ACEStateDTO `json:"new_aces,omitempty"`
}

// ACEStateDTO represents a single ACE with its change status.
type ACEStateDTO struct {
	Position int         `json:"position"`
	ACE      *ACEInfoDTO `json:"ace"`
	Status   string      `json:"status"` // "unchanged", "added", "removed", "modified", "moved"
	MovedTo  int         `json:"moved_to,omitempty"`   // For removed ACEs that moved
	MovedFrom int        `json:"moved_from,omitempty"` // For added ACEs that moved
}

// ACEDiffDTO represents a single ACE change.
type ACEDiffDTO struct {
	Type     string      `json:"type"` // "added", "removed", "modified", "reordered"
	Position int         `json:"position"`
	OldACE   *ACEInfoDTO `json:"old_ace,omitempty"`
	NewACE   *ACEInfoDTO `json:"new_ace,omitempty"`
}

// ACEInfoDTO represents ACE information for display.
type ACEInfoDTO struct {
	TypeName                string   `json:"type_name"`
	TypeCode                uint8    `json:"type_code"`
	Flags                   []string `json:"flags"`
	SID                     *SIDInfo `json:"sid"`
	Mask                    uint32   `json:"mask"`
	MaskFlags               []string `json:"mask_flags"`
	ObjectTypeGUID          string   `json:"object_type_guid,omitempty"`
	InheritedObjectTypeGUID string   `json:"inherited_object_type_guid,omitempty"`
}

// DiffSecurityDescriptors computes the difference between two security descriptors.
func DiffSecurityDescriptors(oldBytes, newBytes []byte, sidResolver resolve.SIDResolver) (*SDDiff, error) {
	if len(oldBytes) == 0 && len(newBytes) == 0 {
		return &SDDiff{HasChanges: false}, nil
	}

	var oldSD, newSD *gontsd.SecurityDescriptor
	var err error

	if len(oldBytes) > 0 {
		oldSD, err = gontsd.Parse(oldBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse old security descriptor: %w", err)
		}
	}

	if len(newBytes) > 0 {
		newSD, err = gontsd.Parse(newBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse new security descriptor: %w", err)
		}
	}

	// Handle case where one is nil
	if oldSD == nil && newSD != nil {
		return &SDDiff{
			HasChanges:   true,
			OwnerChanged: true,
			NewOwner:     sidToInfo(newSD.OwnerSID, sidResolver),
			GroupChanged: true,
			NewGroup:     sidToInfo(newSD.GroupSID, sidResolver),
			DACLDiff:     buildFullACLDiff(nil, newSD.DACL, sidResolver),
		}, nil
	}

	if oldSD != nil && newSD == nil {
		return &SDDiff{
			HasChanges:   true,
			OwnerChanged: true,
			OldOwner:     sidToInfo(oldSD.OwnerSID, sidResolver),
			GroupChanged: true,
			OldGroup:     sidToInfo(oldSD.GroupSID, sidResolver),
			DACLDiff:     buildFullACLDiff(oldSD.DACL, nil, sidResolver),
		}, nil
	}

	// Use gontsd's Compare function
	diff := gontsd.Compare(oldSD, newSD)

	result := &SDDiff{
		HasChanges:          diff.HasChanges(),
		OwnerChanged:        diff.OwnerChanged,
		GroupChanged:        diff.GroupChanged,
		ControlFlagsChanged: diff.ControlFlagsChanged,
	}

	if diff.OwnerChanged {
		result.OldOwner = sidToInfo(diff.OldOwner, sidResolver)
		result.NewOwner = sidToInfo(diff.NewOwner, sidResolver)
	}

	if diff.GroupChanged {
		result.OldGroup = sidToInfo(diff.OldGroup, sidResolver)
		result.NewGroup = sidToInfo(diff.NewGroup, sidResolver)
	}

	if diff.ControlFlagsChanged {
		result.OldControlFlags = diff.OldControlFlags
		result.NewControlFlags = diff.NewControlFlags
	}

	if diff.DACLDiff != nil || oldSD.DACL != nil || newSD.DACL != nil {
		result.DACLDiff = convertACLDiffWithState(diff.DACLDiff, oldSD.DACL, newSD.DACL, sidResolver)
	}

	return result, nil
}

func sidToInfo(sid *gontsd.SID, resolver resolve.SIDResolver) *SIDInfo {
	if sid == nil {
		return nil
	}
	raw := sid.Parsed
	resolved := sid.ResolvedName
	if resolved == "" && resolver != nil {
		if name, err := resolver.Resolve(sid); err == nil {
			resolved = name
		}
	}
	return &SIDInfo{
		Raw:          raw,
		ResolvedName: resolved,
	}
}

func convertACLDiffWithState(aclDiff *gontsd.ACLDiff, oldACL, newACL *gontsd.ACL, resolver resolve.SIDResolver) *ACLDiffDTO {
	dto := &ACLDiffDTO{
		ACEDiffs: make([]ACEDiffDTO, 0),
		OldACEs:  make([]ACEStateDTO, 0),
		NewACEs:  make([]ACEStateDTO, 0),
	}

	if aclDiff != nil {
		dto.RevisionChanged = aclDiff.RevisionChanged
		dto.OldRevision = aclDiff.OldRevision
		dto.NewRevision = aclDiff.NewRevision
	}

	// Build maps of ACE keys to positions for detecting moves
	oldACEPositions := make(map[string]int)
	newACEPositions := make(map[string]int)

	if oldACL != nil {
		for i, ace := range oldACL.ACEs {
			oldACEPositions[aceKey(ace)] = i
		}
	}
	if newACL != nil {
		for i, ace := range newACL.ACEs {
			newACEPositions[aceKey(ace)] = i
		}
	}

	// Build the full OLD ACE list with status annotations
	if oldACL != nil {
		for i, ace := range oldACL.ACEs {
			key := aceKey(ace)
			state := ACEStateDTO{
				Position: i,
				ACE:      aceToInfo(ace, resolver),
				Status:   "unchanged",
			}

			if newPos, existsInNew := newACEPositions[key]; existsInNew {
				if newPos != i {
					state.Status = "moved"
					state.MovedTo = newPos
				}
			} else {
				state.Status = "removed"
			}

			dto.OldACEs = append(dto.OldACEs, state)
		}
	}

	// Build the full NEW ACE list with status annotations
	if newACL != nil {
		for i, ace := range newACL.ACEs {
			key := aceKey(ace)
			state := ACEStateDTO{
				Position: i,
				ACE:      aceToInfo(ace, resolver),
				Status:   "unchanged",
			}

			if oldPos, existsInOld := oldACEPositions[key]; existsInOld {
				if oldPos != i {
					state.Status = "moved"
					state.MovedFrom = oldPos
				}
			} else {
				state.Status = "added"
			}

			dto.NewACEs = append(dto.NewACEs, state)
		}
	}

	// Also build the diff list for summary view
	if aclDiff != nil {
		for _, aceDiff := range aclDiff.ACEDiffs {
			aceDTO := ACEDiffDTO{
				Type:     strings.ToLower(aceDiff.Type.String()),
				Position: aceDiff.Position,
			}
			if aceDiff.OldACE != nil {
				aceDTO.OldACE = aceToInfo(aceDiff.OldACE, resolver)
			}
			if aceDiff.NewACE != nil {
				aceDTO.NewACE = aceToInfo(aceDiff.NewACE, resolver)
			}
			dto.ACEDiffs = append(dto.ACEDiffs, aceDTO)
		}
	}

	return dto
}

func aceKey(ace gontsd.ACE) string {
	sid := ""
	if s := ace.GetSID(); s != nil {
		sid = s.Parsed
	}
	return fmt.Sprintf("%d:%s:%d:%s:%s",
		ace.Type(),
		sid,
		ace.GetMask(),
		ace.GetObjectTypeGUID(),
		ace.GetInheritedObjectTypeGUID(),
	)
}

func aceToInfo(ace gontsd.ACE, resolver resolve.SIDResolver) *ACEInfoDTO {
	return &ACEInfoDTO{
		TypeCode:                ace.Type(),
		TypeName:                aceTypeName(ace.Type()),
		Flags:                   ace.GetFlags(),
		SID:                     sidToInfo(ace.GetSID(), resolver),
		Mask:                    ace.GetMask(),
		MaskFlags:               gontsd.CheckFlags(ace.GetMask()),
		ObjectTypeGUID:          ace.GetObjectTypeGUID(),
		InheritedObjectTypeGUID: ace.GetInheritedObjectTypeGUID(),
	}
}

func aceTypeName(aceType uint8) string {
	names := map[uint8]string{
		0x00: "ACCESS_ALLOWED_ACE",
		0x01: "ACCESS_DENIED_ACE",
		0x02: "SYSTEM_AUDIT_ACE",
		0x03: "SYSTEM_ALARM_ACE",
		0x05: "ACCESS_ALLOWED_OBJECT_ACE",
		0x06: "ACCESS_DENIED_OBJECT_ACE",
		0x07: "SYSTEM_AUDIT_OBJECT_ACE",
		0x08: "SYSTEM_ALARM_OBJECT_ACE",
		0x09: "ACCESS_ALLOWED_CALLBACK_ACE",
		0x0A: "ACCESS_DENIED_CALLBACK_ACE",
		0x0B: "ACCESS_ALLOWED_CALLBACK_OBJECT_ACE",
		0x0C: "ACCESS_DENIED_CALLBACK_OBJECT_ACE",
		0x0D: "SYSTEM_AUDIT_CALLBACK_ACE",
		0x0E: "SYSTEM_ALARM_CALLBACK_ACE",
		0x0F: "SYSTEM_AUDIT_CALLBACK_OBJECT_ACE",
		0x10: "SYSTEM_ALARM_CALLBACK_OBJECT_ACE",
		0x11: "SYSTEM_MANDATORY_LABEL_ACE",
		0x12: "SYSTEM_RESOURCE_ATTRIBUTE_ACE",
		0x13: "SYSTEM_SCOPED_POLICY_ID_ACE",
	}
	if name, ok := names[aceType]; ok {
		return name
	}
	return fmt.Sprintf("UNKNOWN_ACE_TYPE_%02x", aceType)
}

func buildFullACLDiff(oldACL, newACL *gontsd.ACL, resolver resolve.SIDResolver) *ACLDiffDTO {
	dto := &ACLDiffDTO{
		ACEDiffs: []ACEDiffDTO{},
		OldACEs:  []ACEStateDTO{},
		NewACEs:  []ACEStateDTO{},
	}

	if oldACL == nil && newACL == nil {
		return nil
	}

	if oldACL != nil && newACL == nil {
		dto.RevisionChanged = true
		dto.OldRevision = oldACL.Revision
		for i, ace := range oldACL.ACEs {
			dto.ACEDiffs = append(dto.ACEDiffs, ACEDiffDTO{
				Type:     "removed",
				Position: i,
				OldACE:   aceToInfo(ace, resolver),
			})
			dto.OldACEs = append(dto.OldACEs, ACEStateDTO{
				Position: i,
				ACE:      aceToInfo(ace, resolver),
				Status:   "removed",
			})
		}
		return dto
	}

	if oldACL == nil && newACL != nil {
		dto.RevisionChanged = true
		dto.NewRevision = newACL.Revision
		for i, ace := range newACL.ACEs {
			dto.ACEDiffs = append(dto.ACEDiffs, ACEDiffDTO{
				Type:     "added",
				Position: i,
				NewACE:   aceToInfo(ace, resolver),
			})
			dto.NewACEs = append(dto.NewACEs, ACEStateDTO{
				Position: i,
				ACE:      aceToInfo(ace, resolver),
				Status:   "added",
			})
		}
		return dto
	}

	return dto
}
