// API response types

export interface ADObject {
  id: string;
  dn: string;
  type: string;
  guid: string;
}

export interface ObjectsResponse {
  objects: ADObject[];
  total: number;
}

export interface TimelineEntry {
  usn_changed: number;
  timestamp: string;
  modified_by?: string;
}

export interface AttributeChange {
  attribute: string;
  old_value: unknown;
  new_value: unknown;
  is_single_valued: boolean;
}

export interface SIDInfo {
  raw: string;
  resolved_name?: string;
}

export interface ACE {
  type_name?: string;
  type_code?: number;
  flags?: string[];
  sid?: SIDInfo;
  mask?: number;
  mask_flags?: string[];
  object_type_guid?: string;
  inherited_object_type_guid?: string;
}

export type ACEStatusType = 'added' | 'removed' | 'moved' | 'unchanged';

export interface ACEState {
  position: number;
  status: ACEStatusType;
  ace?: ACE;
  moved_to?: number;
  moved_from?: number;
}

export interface ACLDiff {
  old_aces?: ACEState[];
  new_aces?: ACEState[];
}

export interface SDDiffResponse {
  has_changes: boolean;
  owner_changed?: boolean;
  group_changed?: boolean;
  control_flags_changed?: boolean;
  old_owner?: SIDInfo;
  new_owner?: SIDInfo;
  old_group?: SIDInfo;
  new_group?: SIDInfo;
  old_control_flags?: number;
  new_control_flags?: number;
  dacl_diff?: ACLDiff;
}

export interface FetchObjectsParams {
  type?: string;
  search?: string;
  limit?: number;
  offset?: number;
}

// Shared component types
export interface ChangeCounts {
  added: number;
  removed: number;
  moved: number;
  unchanged: number;
}
