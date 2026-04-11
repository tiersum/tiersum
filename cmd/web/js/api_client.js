/** TierSum REST client (`/api/v1`). */

export const apiClient = {
    baseURL: '',

    async request(endpoint, options = {}) {
        const url = `${this.baseURL}${endpoint}`;
        const config = {
            headers: {
                'Content-Type': 'application/json',
                ...options.headers
            },
            ...options
        };

        try {
            const response = await fetch(url, config);
            if (!response.ok) {
                const error = await response.json().catch(() => ({}));
                const msg = error.error || error.message || response.statusText || `HTTP ${response.status}`;
                throw new Error(typeof msg === 'string' ? msg : JSON.stringify(msg));
            }
            return await response.json();
        } catch (error) {
            console.error('API request failed:', error);
            throw error;
        }
    },

    getDocuments: () => apiClient.request('/api/v1/documents').then(r => r.documents || []),
    getDocument: (id) => apiClient.request(`/api/v1/documents/${id}`),
    getDocumentSummaries: (id) => apiClient.request(`/api/v1/documents/${id}/summaries`).then(r => r.summaries || []),
    getDocumentChapters: (id) => apiClient.request(`/api/v1/documents/${id}/chapters`).then(r => r.chapters || []),
    createDocument: (data) => apiClient.request('/api/v1/documents', { method: 'POST', body: JSON.stringify(data) }),

    progressiveQuery: (question) => apiClient.request('/api/v1/query/progressive', {
        method: 'POST',
        body: JSON.stringify({ question, max_results: 100 })
    }),

    getTags: () => apiClient.request('/api/v1/tags').then(r => r.tags || []),
    getTagGroups: () => apiClient.request('/api/v1/tags/groups').then(r => r.groups || []),
    triggerTagGrouping: () => apiClient.request('/api/v1/tags/group', { method: 'POST' }),
};
