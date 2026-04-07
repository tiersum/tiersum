const API_BASE = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'

// Types
export interface QueryItem {
  id: string
  title: string
  content: string
  tier: string
  path: string
  relevance: number
}

export interface ProgressiveQueryResponse {
  question: string
  steps: Array<{
    step: string
    input: unknown
    output: unknown
    duration_ms: number
  }>
  results: QueryItem[]
}

export interface Document {
  id: string
  title: string
  content: string
  format: string
  tags: string[]
  status: string
  hot_score: number
  query_count: number
  created_at: string
}

export interface SummaryNode {
  id: string
  document_id: string
  tier: string
  content: string
  path: string
  created_at: string
}

export interface TagGroup {
  id: string
  name: string
  description: string
  tags: string[]
}

export interface Tag {
  id: string
  name: string
  group_id: string
  document_count: number
}

// API Client
async function fetchAPI<T>(path: string, options?: RequestInit): Promise<T> {
  const url = `${API_BASE}${path}`
  const res = await fetch(url, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...options?.headers,
    },
  })
  if (!res.ok) {
    const error = await res.text()
    throw new Error(`API error: ${res.status} - ${error}`)
  }
  return res.json()
}

export const api = {
  // Progressive Query
  progressiveQuery: async (question: string, maxResults = 100) => {
    const res = await fetchAPI<ProgressiveQueryResponse>('/api/v1/query/progressive', {
      method: 'POST',
      body: JSON.stringify({ question, max_results: maxResults }),
    })
    return res
  },

  // Documents
  getDocument: async (id: string) => {
    const res = await fetchAPI<{ document?: Document } & Document>(`/api/v1/documents/${id}`)
    // Handle both wrapped and unwrapped responses
    return ('document' in res && res.document) ? res.document : res as Document
  },

  getDocumentSummaries: async (id: string) => {
    const res = await fetchAPI<{ summaries: SummaryNode[] }>(`/api/v1/documents/${id}/summaries`)
    return res.summaries
  },

  // Tags
  getTags: async () => {
    const res = await fetchAPI<{ tags: Tag[] }>('/api/v1/tags')
    return res.tags
  },

  getTagGroups: async () => {
    const res = await fetchAPI<{ groups: TagGroup[] }>('/api/v1/tags/groups')
    return res.groups
  },

  triggerTagGrouping: () =>
    fetchAPI<{message: string}>('/api/v1/tags/group', { method: 'POST' }),
}
