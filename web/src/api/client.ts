import type {
  ApiErrorBody,
  ListResponse,
  Offer,
  StreamerPublic,
  StreamerSettings,
  TokenResponse,
  User,
} from './types'

const STORAGE_ACCESS = 'vo_access_token'
const STORAGE_REFRESH = 'vo_refresh_token'

export class ApiClientError extends Error {
  code: string
  status: number
  details?: Record<string, unknown>

  constructor(status: number, body: ApiErrorBody['error']) {
    super(body.message)
    this.name = 'ApiClientError'
    this.code = body.code
    this.status = status
    this.details = body.details
  }
}

type RequestOptions = Omit<RequestInit, 'body'> & {
  body?: unknown
  auth?: boolean
}

let refreshPromise: Promise<boolean> | null = null

function getAccessToken(): string | null {
  return localStorage.getItem(STORAGE_ACCESS)
}

function getRefreshToken(): string | null {
  return localStorage.getItem(STORAGE_REFRESH)
}

export function setTokens(access: string, refresh: string): void {
  localStorage.setItem(STORAGE_ACCESS, access)
  localStorage.setItem(STORAGE_REFRESH, refresh)
}

export function clearTokens(): void {
  localStorage.removeItem(STORAGE_ACCESS)
  localStorage.removeItem(STORAGE_REFRESH)
}

export function hasStoredSession(): boolean {
  return Boolean(getAccessToken() && getRefreshToken())
}

async function parseError(res: Response): Promise<ApiClientError> {
  try {
    const body = (await res.json()) as ApiErrorBody
    if (body?.error) {
      return new ApiClientError(res.status, body.error)
    }
  } catch {
    // ignore
  }
  return new ApiClientError(res.status, {
    code: 'unknown',
    message: res.statusText || 'Неизвестная ошибка',
  })
}

async function refreshSession(): Promise<boolean> {
  const refresh = getRefreshToken()
  if (!refresh) return false

  const res = await fetch('/api/v1/auth/refresh', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ refresh_token: refresh }),
  })

  if (!res.ok) {
    clearTokens()
    return false
  }

  const data = (await res.json()) as TokenResponse
  setTokens(data.access_token, data.refresh_token)
  return true
}

async function request<T>(path: string, opts: RequestOptions = {}): Promise<T> {
  const headers = new Headers(opts.headers)
  if (opts.body !== undefined) {
    headers.set('Content-Type', 'application/json')
  }
  if (opts.auth !== false) {
    const token = getAccessToken()
    if (token) headers.set('Authorization', `Bearer ${token}`)
  }

  const doFetch = () =>
    fetch(path, {
      ...opts,
      headers,
      body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
    })

  let res = await doFetch()

  if (res.status === 401 && opts.auth !== false && getRefreshToken()) {
    if (!refreshPromise) {
      refreshPromise = refreshSession().finally(() => {
        refreshPromise = null
      })
    }
    const ok = await refreshPromise
    if (ok) {
      const token = getAccessToken()
      if (token) headers.set('Authorization', `Bearer ${token}`)
      res = await doFetch()
    }
  }

  if (!res.ok) {
    throw await parseError(res)
  }

  if (res.status === 204) {
    return undefined as T
  }

  return (await res.json()) as T
}

export const api = {
  register(email: string, username: string, password: string) {
    return request<TokenResponse>('/api/v1/auth/register', {
      method: 'POST',
      auth: false,
      body: { email, username, password },
    })
  },

  login(email: string, password: string) {
    return request<TokenResponse>('/api/v1/auth/login', {
      method: 'POST',
      auth: false,
      body: { email, password },
    })
  },

  logout() {
    const refresh = getRefreshToken()
    if (!refresh) return Promise.resolve()
    return request<void>('/api/v1/auth/logout', {
      method: 'POST',
      body: { refresh_token: refresh },
    }).finally(clearTokens)
  },

  me() {
    return request<User>('/api/v1/me')
  },

  updateMe(body: { display_name?: string; avatar_url?: string; role?: string }) {
    return request<User>('/api/v1/me', { method: 'PATCH', body })
  },

  getSettings() {
    return request<StreamerSettings>('/api/v1/me/settings')
  },

  updateSettings(body: { accepting_offers?: boolean }) {
    return request<StreamerSettings>('/api/v1/me/settings', { method: 'PATCH', body })
  },

  listStreamers(params?: { q?: string; limit?: number; cursor?: string }) {
    const search = new URLSearchParams()
    if (params?.q) search.set('q', params.q)
    if (params?.limit) search.set('limit', String(params.limit))
    if (params?.cursor) search.set('cursor', params.cursor)
    const qs = search.toString()
    return request<ListResponse<StreamerPublic>>(
      `/api/v1/streamers${qs ? `?${qs}` : ''}`,
      { auth: false },
    )
  },

  getStreamer(username: string) {
    return request<StreamerPublic>(`/api/v1/streamers/${encodeURIComponent(username)}`, {
      auth: false,
    })
  },

  createOffer(username: string, url: string, comment: string) {
    return request<Offer>(`/api/v1/streamers/${encodeURIComponent(username)}/offers`, {
      method: 'POST',
      body: { url, comment },
    })
  },

  listQueue(params?: { status?: string; limit?: number; cursor?: string }) {
    const search = new URLSearchParams()
    if (params?.status) search.set('status', params.status)
    if (params?.limit) search.set('limit', String(params.limit))
    if (params?.cursor) search.set('cursor', params.cursor)
    const qs = search.toString()
    return request<ListResponse<Offer>>(`/api/v1/me/offers${qs ? `?${qs}` : ''}`)
  },

  updateOfferStatus(id: string, status: string) {
    return request<Offer>(`/api/v1/me/offers/${id}`, {
      method: 'PATCH',
      body: { status },
    })
  },

  deleteOffer(id: string) {
    return request<void>(`/api/v1/me/offers/${id}`, { method: 'DELETE' })
  },

  listSent(params?: { limit?: number; cursor?: string }) {
    const search = new URLSearchParams()
    if (params?.limit) search.set('limit', String(params.limit))
    if (params?.cursor) search.set('cursor', params.cursor)
    const qs = search.toString()
    return request<ListResponse<Offer>>(`/api/v1/me/sent${qs ? `?${qs}` : ''}`)
  },

  revokeSent(id: string) {
    return request<void>(`/api/v1/me/sent/${id}`, { method: 'DELETE' })
  },
}
