const API_ROOT = '/api/v1'

export class APIError extends Error {
  constructor(public code: string, message: string, public requestID: string, public status: number) {
    super(message)
  }
}

export async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const token = sessionStorage.getItem('access_token')
  const headers = new Headers(options.headers)
  headers.set('Accept', 'application/json')
  if (options.body) headers.set('Content-Type', 'application/json')
  if (token) headers.set('Authorization', `Bearer ${token}`)
  const response = await fetch(`${API_ROOT}${path}`, { ...options, headers })
  if (response.status === 204) return undefined as T
  const payload = await response.json()
  if (!response.ok) throw new APIError(payload.code ?? 'REQUEST_FAILED', payload.message ?? 'Request failed', payload.request_id ?? '', response.status)
  return payload.data as T
}
