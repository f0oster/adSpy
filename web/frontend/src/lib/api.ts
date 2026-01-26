import type {
  ObjectsResponse,
  TimelineEntry,
  AttributeChange,
  SDDiffResponse,
  FetchObjectsParams,
} from './types';

const API_BASE = import.meta.env.VITE_API_BASE || '/api';

export class ApiError extends Error {
  constructor(
    message: string,
    public status: number,
    public endpoint: string
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

async function handleResponse<T>(response: Response, endpoint: string): Promise<T> {
  if (!response.ok) {
    const message = response.status === 404
      ? 'Resource not found'
      : response.status >= 500
        ? 'Server error - please try again'
        : `Request failed (${response.status})`;
    throw new ApiError(message, response.status, endpoint);
  }
  try {
    return await response.json();
  } catch {
    throw new ApiError('Invalid response format', response.status, endpoint);
  }
}

export async function fetchObjects(params: FetchObjectsParams = {}): Promise<ObjectsResponse> {
  const { type = '', search = '', limit = 50, offset = 0 } = params;
  const urlParams = new URLSearchParams();
  if (type) urlParams.set('type', type);
  if (search) urlParams.set('search', search);
  urlParams.set('limit', limit.toString());
  urlParams.set('offset', offset.toString());

  const endpoint = `${API_BASE}/objects?${urlParams}`;
  const response = await fetch(endpoint);
  return handleResponse<ObjectsResponse>(response, endpoint);
}

export async function fetchObjectTimeline(id: string): Promise<TimelineEntry[]> {
  const endpoint = `${API_BASE}/objects/${id}/timeline`;
  const response = await fetch(endpoint);
  return handleResponse<TimelineEntry[]>(response, endpoint);
}

export async function fetchVersionChanges(objectId: string, usn: number): Promise<AttributeChange[]> {
  const endpoint = `${API_BASE}/objects/${objectId}/versions/${usn}/changes`;
  const response = await fetch(endpoint);
  return handleResponse<AttributeChange[]>(response, endpoint);
}

export async function fetchSDDiff(oldValue: string, newValue: string): Promise<SDDiffResponse> {
  const endpoint = `${API_BASE}/sddiff`;
  const response = await fetch(endpoint, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ old_value: oldValue, new_value: newValue }),
  });
  return handleResponse<SDDiffResponse>(response, endpoint);
}

export async function fetchObjectTypes(): Promise<string[]> {
  const endpoint = `${API_BASE}/object-types`;
  const response = await fetch(endpoint);
  return handleResponse<string[]>(response, endpoint);
}
