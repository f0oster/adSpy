export function extractName(dn: string): string {
  const match = dn.match(/^([A-Za-z]+)=([^,]+)/i);
  return match ? match[2] : dn.split(',')[0] || dn;
}

export function extractType(objectType: string | undefined): string {
  if (!objectType) return 'unknown';
  const match = objectType.match(/^CN=([^,]+)/i);
  return match ? match[1] : objectType;
}

export function formatDate(dateStr: string | undefined): string {
  if (!dateStr) return '';
  const date = new Date(dateStr);
  return date.toLocaleString('en-US', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

export function formatFullDate(dateStr: string | undefined): string {
  if (!dateStr) return '';
  return new Date(dateStr).toLocaleString();
}

export function formatValue(value: unknown): string {
  if (value === null || value === undefined) {
    return '(none)';
  }
  if (Array.isArray(value)) {
    if (value.length === 0) return '(empty)';
    if (value.length === 1) return String(value[0]);
    return value.map(String).join(', ');
  }
  if (typeof value === 'object') {
    return JSON.stringify(value, null, 2);
  }
  return String(value);
}

const SECURITY_DESCRIPTOR_ATTR = 'ntsecuritydescriptor';

export function isSecurityDescriptor(attr: string): boolean {
  return attr.toLowerCase() === SECURITY_DESCRIPTOR_ATTR;
}

export function getBase64Value(value: unknown): string {
  if (typeof value === 'string') {
    return value;
  }
  if (Array.isArray(value) && value.length > 0) {
    return String(value[0]);
  }
  return '';
}

export interface ArrayDiffResult {
  added: string[];
  removed: string[];
  unchanged: string[];
}

function isMultiValuedChange(oldValue: unknown, newValue: unknown): boolean {
  const oldLen = Array.isArray(oldValue) ? oldValue.length : 0;
  const newLen = Array.isArray(newValue) ? newValue.length : 0;
  return oldLen > 1 || newLen > 1 || (oldLen >= 1 && newLen >= 1);
}

export function shouldShowAsMultiValued(change: { is_single_valued?: boolean; old_value: unknown; new_value: unknown }): boolean {
  if (typeof change.is_single_valued === 'boolean') {
    return !change.is_single_valued;
  }
  return isMultiValuedChange(change.old_value, change.new_value);
}

export function computeArrayDiff(oldValue: unknown, newValue: unknown): ArrayDiffResult {
  const oldArray = normalizeToStringArray(oldValue);
  const newArray = normalizeToStringArray(newValue);

  const oldSet = new Set(oldArray);
  const newSet = new Set(newArray);

  const added: string[] = [];
  const removed: string[] = [];
  const unchangedSet = new Set<string>();

  for (const item of oldArray) {
    if (newSet.has(item)) {
      unchangedSet.add(item);
    } else {
      removed.push(item);
    }
  }

  for (const item of newArray) {
    if (!oldSet.has(item)) {
      added.push(item);
    }
  }

  return { added, removed, unchanged: [...unchangedSet] };
}

function normalizeToStringArray(value: unknown): string[] {
  if (value === null || value === undefined) {
    return [];
  }
  if (Array.isArray(value)) {
    return value.map(String);
  }
  return [String(value)];
}

export function isDN(value: string): boolean {
  return value.includes('=') && value.includes(',');
}

export function formatDNValue(value: string): string {
  return isDN(value) ? extractName(value) : value;
}

export function getErrorMessage(error: unknown): string {
  return error instanceof Error ? error.message : 'Unknown error';
}
