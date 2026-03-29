package sddiff

import (
	"github.com/f0oster/gontsd"
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
	DACLDiff            *ACLDiffDTO `json:"dacl_diff,omitempty"`
	HasChanges          bool        `json:"has_changes"`
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
	Position  int         `json:"position"`
	ACE       *ACEInfoDTO `json:"ace"`
	Status    string      `json:"status"` // "unchanged", "added", "removed", "modified", "moved"
	MovedTo   int         `json:"moved_to,omitempty"`
	MovedFrom int         `json:"moved_from,omitempty"`
}

// ACEDiffDTO represents a single ACE change.
type ACEDiffDTO struct {
	Type            string      `json:"type"` // "added", "removed", "modified", "reordered", "modified_reordered"
	OldPosition     int         `json:"old_position"`
	NewPosition     int         `json:"new_position"`
	OldACE          *ACEInfoDTO `json:"old_ace,omitempty"`
	NewACE          *ACEInfoDTO `json:"new_ace,omitempty"`
	AddedRights     []string    `json:"added_rights,omitempty"`
	RemovedRights   []string    `json:"removed_rights,omitempty"`
	UnchangedRights []string    `json:"unchanged_rights,omitempty"`
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
func DiffSecurityDescriptors(oldBytes, newBytes []byte, resolver *gontsd.Resolver) (*SDDiff, error) {
	if len(oldBytes) == 0 && len(newBytes) == 0 {
		return &SDDiff{HasChanges: false}, nil
	}

	var oldSD, newSD *gontsd.SecurityDescriptor
	var err error

	if len(oldBytes) > 0 {
		oldSD, err = gontsd.Parse(oldBytes, resolver)
		if err != nil {
			return nil, err
		}
	}

	if len(newBytes) > 0 {
		newSD, err = gontsd.Parse(newBytes, resolver)
		if err != nil {
			return nil, err
		}
	}

	diff := gontsd.Compare(oldSD, newSD)

	result := &SDDiff{
		HasChanges:          diff.HasChanges(),
		OwnerChanged:        diff.OwnerChanged,
		GroupChanged:        diff.GroupChanged,
		ControlFlagsChanged: diff.ControlFlagsChanged,
	}

	if diff.OwnerChanged {
		result.OldOwner = sidToInfo(diff.OldOwner)
		result.NewOwner = sidToInfo(diff.NewOwner)
	}

	if diff.GroupChanged {
		result.OldGroup = sidToInfo(diff.OldGroup)
		result.NewGroup = sidToInfo(diff.NewGroup)
	}

	if diff.ControlFlagsChanged {
		result.OldControlFlags = uint16(diff.OldControlFlags)
		result.NewControlFlags = uint16(diff.NewControlFlags)
	}

	if diff.DACLDiff != nil {
		var oldDACL, newDACL *gontsd.ACL
		if oldSD != nil {
			oldDACL = oldSD.DACL
		}
		if newSD != nil {
			newDACL = newSD.DACL
		}
		result.DACLDiff = convertACLDiff(diff.DACLDiff, oldDACL, newDACL)
	}

	return result, nil
}

func sidToInfo(sid *gontsd.SID) *SIDInfo {
	if sid == nil {
		return nil
	}
	return &SIDInfo{
		Raw:          sid.Value,
		ResolvedName: sid.Resolved(),
	}
}

func convertACLDiff(aclDiff *gontsd.ACLDiff, oldACL, newACL *gontsd.ACL) *ACLDiffDTO {
	dto := &ACLDiffDTO{
		RevisionChanged: aclDiff.RevisionChanged,
		OldRevision:     aclDiff.OldRevision,
		NewRevision:     aclDiff.NewRevision,
		ACEDiffs:        make([]ACEDiffDTO, 0, len(aclDiff.ACEDiffs)),
	}

	oldStatus := map[int]ACEStateDTO{}
	newStatus := map[int]ACEStateDTO{}

	for _, aceDiff := range aclDiff.ACEDiffs {
		aceDTO := ACEDiffDTO{
			OldPosition: aceDiff.OldPosition,
			NewPosition: aceDiff.NewPosition,
		}

		dt := aceDiff.Type
		switch {
		case dt.Has(gontsd.DiffModified) && dt.Has(gontsd.DiffReordered):
			aceDTO.Type = "modified_reordered"
		case dt.Has(gontsd.DiffModified):
			aceDTO.Type = "modified"
		case dt.Has(gontsd.DiffReordered):
			aceDTO.Type = "reordered"
		case dt.Has(gontsd.DiffAdded):
			aceDTO.Type = "added"
		case dt.Has(gontsd.DiffRemoved):
			aceDTO.Type = "removed"
		}

		if aceDiff.OldACE != nil {
			aceDTO.OldACE = aceToInfo(aceDiff.OldACE)
		}
		if aceDiff.NewACE != nil {
			aceDTO.NewACE = aceToInfo(aceDiff.NewACE)
		}

		aceDTO.AddedRights, aceDTO.RemovedRights, aceDTO.UnchangedRights = aceDiff.CompareAccessRights()

		dto.ACEDiffs = append(dto.ACEDiffs, aceDTO)

		// Track state for full ACE list annotations
		if dt.Has(gontsd.DiffRemoved) {
			oldStatus[aceDiff.OldPosition] = ACEStateDTO{Status: "removed"}
		} else if dt.Has(gontsd.DiffAdded) {
			newStatus[aceDiff.NewPosition] = ACEStateDTO{Status: "added"}
		} else if dt.Has(gontsd.DiffModified) || dt.Has(gontsd.DiffReordered) {
			oldStatus[aceDiff.OldPosition] = ACEStateDTO{Status: "moved", MovedTo: aceDiff.NewPosition}
			newStatus[aceDiff.NewPosition] = ACEStateDTO{Status: "moved", MovedFrom: aceDiff.OldPosition}
		}
	}

	dto.OldACEs = annotateACEs(oldACL, oldStatus)
	dto.NewACEs = annotateACEs(newACL, newStatus)

	return dto
}

func annotateACEs(acl *gontsd.ACL, status map[int]ACEStateDTO) []ACEStateDTO {
	if acl == nil {
		return nil
	}
	result := make([]ACEStateDTO, len(acl.ACEs))
	for i, ace := range acl.ACEs {
		if s, ok := status[i]; ok {
			s.Position = i
			s.ACE = aceToInfo(ace)
			result[i] = s
		} else {
			result[i] = ACEStateDTO{Position: i, ACE: aceToInfo(ace), Status: "unchanged"}
		}
	}
	return result
}

func aceToInfo(ace gontsd.ACE) *ACEInfoDTO {
	info := &ACEInfoDTO{
		TypeCode:  uint8(ace.Type()),
		TypeName:  ace.Type().String(),
		Flags:     ace.AceFlags().Names(),
		SID:       sidToInfo(ace.SID()),
		Mask:      uint32(ace.Mask()),
		MaskFlags: ace.Mask().Names(),
	}
	if g := ace.ObjectTypeGUID(); g != nil {
		info.ObjectTypeGUID = g.Resolved()
	}
	if g := ace.InheritedObjectTypeGUID(); g != nil {
		info.InheritedObjectTypeGUID = g.Resolved()
	}
	return info
}
