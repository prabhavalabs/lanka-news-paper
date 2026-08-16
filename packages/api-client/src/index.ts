export type SourceType =
  | 'private_media'
  | 'state_owned'
  | 'government'
  | 'independent'
  | 'international'
  | 'other'

export type PublicSource = {
  id: string
  name: string
  type: SourceType
}

export type PublicCategory = {
  slug: string
  name_si: string
}

export type PublicArticle = {
  id: string
  headline: string
  source: PublicSource
  category: PublicCategory | null
  published_at: string
  received_at: string
  original_url: string
  excerpt: string | null
  media: string | null
  event_id: string | null
}

export type CursorPage<T> = {
  items: T[]
  next_cursor: string | null
}

export function createClient(baseUrl = '') {
  return {
    async listNews(): Promise<CursorPage<PublicArticle>> {
      const response = await fetch(`${baseUrl}/api/v1/news`)
      if (!response.ok) {
        throw new Error(`list news failed: ${response.status}`)
      }
      return response.json() as Promise<CursorPage<PublicArticle>>
    },
    async live(): Promise<{ status: string }> {
      const response = await fetch(`${baseUrl}/api/v1/health/live`)
      if (!response.ok) {
        throw new Error(`health failed: ${response.status}`)
      }
      return response.json() as Promise<{ status: string }>
    },
  }
}
