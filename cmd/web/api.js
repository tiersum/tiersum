// API Client
const API_BASE = ''

const api = {
    async fetchAPI(path, options = {}) {
        const url = `${API_BASE}${path}`
        const res = await fetch(url, {
            ...options,
            headers: {
                'Content-Type': 'application/json',
                ...options.headers,
            },
        })
        if (!res.ok) {
            const error = await res.text()
            throw new Error(`API error: ${res.status} - ${error}`)
        }
        return res.json()
    },

    // Progressive Query
    progressiveQuery: async (question, maxResults = 100) => {
        const res = await api.fetchAPI('/api/v1/query/progressive', {
            method: 'POST',
            body: JSON.stringify({ question, max_results: maxResults }),
        })
        return res
    },

    // Documents
    getDocuments: async () => {
        const res = await api.fetchAPI('/api/v1/documents')
        return res.documents || []
    },

    createDocument: async (doc) => {
        const res = await api.fetchAPI('/api/v1/documents', {
            method: 'POST',
            body: JSON.stringify(doc),
        })
        return res
    },

    getDocument: async (id) => {
        const res = await api.fetchAPI(`/api/v1/documents/${id}`)
        return res.document || res
    },

    getDocumentSummaries: async (id) => {
        const res = await api.fetchAPI(`/api/v1/documents/${id}/summaries`)
        return res.summaries || []
    },

    updateDocument: async (id, doc) => {
        const res = await api.fetchAPI(`/api/v1/documents/${id}`, {
            method: 'PUT',
            body: JSON.stringify(doc),
        })
        return res
    },

    deleteDocument: async (id) => {
        await api.fetchAPI(`/api/v1/documents/${id}`, {
            method: 'DELETE',
        })
    },

    // Tags
    getTags: async () => {
        const res = await api.fetchAPI('/api/v1/tags')
        return res.tags || []
    },

    getTagGroups: async () => {
        const res = await api.fetchAPI('/api/v1/tags/groups')
        return res.groups || []
    },

    triggerTagGrouping: () =>
        api.fetchAPI('/api/v1/tags/group', { method: 'POST' }),
}

export { api }
