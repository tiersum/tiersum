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

    /** Plain-text GET (e.g. Prometheus exposition at /api/v1/metrics). */
    async requestText(endpoint, options = {}) {
        const url = `${this.baseURL}${endpoint}`;
        const response = await fetch(url, {
            headers: {
                Accept: 'text/plain; version=0.0.4, */*',
                ...options.headers
            },
            ...options
        });
        if (!response.ok) {
            const t = await response.text().catch(() => '');
            throw new Error(t || response.statusText || `HTTP ${response.status}`);
        }
        return await response.text();
    },

    getDocuments: () => apiClient.request('/api/v1/documents').then(r => r.documents || []),
    getDocument: (id) => apiClient.request(`/api/v1/documents/${id}`),
    getDocumentSummaries: (id) => apiClient.request(`/api/v1/documents/${id}/summaries`).then(r => r.summaries || []),
    getDocumentChapters: (id) => apiClient.request(`/api/v1/documents/${id}/chapters`).then(r => r.chapters || []),
    createDocument: (data) => apiClient.request('/api/v1/documents', { method: 'POST', body: JSON.stringify(data) }),

    /**
     * @param {string} question
     * @param {{ max_results?: number, trace?: boolean }} [options] trace: when true, append debug_trace=1 so the HTTP root span is force-sampled (OpenTelemetry).
     */
    progressiveQuery: (question, options = {}) => {
        const max = options.max_results != null ? options.max_results : 100;
        const payload = { question, max_results: max };
        let path = '/api/v1/query/progressive';
        if (options.trace) {
            path += '?debug_trace=1';
        }
        return apiClient.request(path, {
            method: 'POST',
            body: JSON.stringify(payload)
        });
    },

    getTags: () => apiClient.request('/api/v1/tags').then(r => r.tags || []),
    getTagGroups: () => apiClient.request('/api/v1/tags/groups').then(r => r.groups || []),
    triggerTagGrouping: () => apiClient.request('/api/v1/tags/group', { method: 'POST' }),

    getMonitoring: () => apiClient.request('/api/v1/monitoring'),
    getMetricsText: () => apiClient.requestText('/api/v1/metrics'),

    /** @param {{ limit?: number, offset?: number }} [params] */
    listTraces: (params = {}) => {
        const q = new URLSearchParams();
        if (params.limit != null) q.set('limit', String(params.limit));
        if (params.offset != null) q.set('offset', String(params.offset));
        const s = q.toString();
        return apiClient.request('/api/v1/traces' + (s ? `?${s}` : ''));
    },

    getTrace: (traceId) =>
        apiClient.request(`/api/v1/traces/${encodeURIComponent(traceId)}`),
};
