export type UserRole = 'viewer' | 'streamer'

export type OfferStatus = 'pending' | 'watched' | 'skipped' | 'rejected'

export interface ApiErrorBody {
  error: {
    code: string
    message: string
    details?: Record<string, unknown>
  }
}

export interface User {
  id: string
  email: string
  username: string
  role: UserRole
  display_name: string
  avatar_url: string
  created_at: string
  updated_at: string
}

export interface TokenResponse {
  access_token: string
  refresh_token: string
  expires_in: number
}

export interface StreamerPublic {
  username: string
  display_name: string
  avatar_url: string
  accepting_offers: boolean
}

export interface StreamerSettings {
  accepting_offers: boolean
  allow_anonymous: boolean
  min_account_age_seconds: number
}

export interface Offer {
  id: string
  sender_id?: string
  url: string
  normalized_url: string
  provider: string
  external_id: string
  title: string
  thumbnail_url: string
  duration_seconds?: number
  comment: string
  status: OfferStatus
  watched_at?: string
  created_at: string
}

export interface ListResponse<T> {
  items: T[]
  next_cursor?: string
}
