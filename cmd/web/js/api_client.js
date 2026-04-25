/** TierSum BFF REST client (`/bff/v1`) — uses session cookies (credentials: include). */

/** True when the human-track profile role is administrator (case-insensitive). */
export function isBrowserAdminRole(role) {
    return String(role ?? '')
        .trim()
        .toLowerCase() === 'admin';
}

/** True when the human-track profile role is viewer (read-only BFF; case-insensitive). */
export function isBrowserViewerRole(role) {
    return String(role ?? '')
        .trim()
        .toLowerCase() === 'viewer';
}

/** BFF observability surfaces (`/monitoring`, `/traces`) are admin-only. */
export function canAccessObservabilityBFF(role) {
    return isBrowserAdminRole(role);
}

/** Redirect helper: never force a page reload (avoids races with Vue Router guards).
 *  Auth flow is driven by router.beforeEach in main.js; this helper only logs. */
function redirectAuth(endpoint, status, errBody) {
    if (status === 403 && errBody && errBody.code === 'SYSTEM_NOT_INITIALIZED') {
        console.warn('System not initialized; router guard will redirect to /init');
        return;
    }
    if (status === 401 && endpoint.startsWith('/bff/v1')) {
        const open = [
            '/bff/v1/system/status',
            '/bff/v1/system/bootstrap',
            '/bff/v1/auth/login',
            '/bff/v1/auth/device_login',
            '/bff/v1/auth/logout'
        ];
        if (open.some((p) => endpoint.startsWith(p))) return;
        console.warn('Unauthorized BFF request; router guard will redirect to /login');
    }
}

export const apiClient = {
    baseURL: '',

    async request(endpoint, options = {}) {
        const url = `${this.baseURL}${endpoint}`;
        const config = {
            credentials: 'include',
            headers: {
                'Content-Type': 'application/json',
                ...options.headers
            },
            ...options
        };

        try {
            const response = await fetch(url, config);
            const ct = response.headers.get('content-type') || '';
            let errBody = {};
            if (!response.ok && ct.includes('application/json')) {
                errBody = await response.json().catch(() => ({}));
            }
            redirectAuth(endpoint, response.status, errBody);

            if (!response.ok) {
                if (!ct.includes('application/json')) {
                    const t = await response.text().catch(() => '');
                    throw new Error(t || response.statusText || `HTTP ${response.status}`);
                }
                const msg = errBody.error || errBody.message || response.statusText || `HTTP ${response.status}`;
                throw new Error(typeof msg === 'string' ? msg : JSON.stringify(msg));
            }
            if (ct.includes('application/json')) {
                return await response.json();
            }
            return await response.text();
        } catch (error) {
            console.error('API request failed:', error);
            throw error;
        }
    },

    /** Plain-text GET (e.g. /metrics). */
    async requestText(endpoint, options = {}) {
        const url = `${this.baseURL}${endpoint}`;
        const response = await fetch(url, {
            credentials: 'include',
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

    async getSystemStatus() {
        const url = `${this.baseURL}/bff/v1/system/status`;
        const r = await fetch(url, { credentials: 'omit' });
        return r.json();
    },

    bootstrap(username) {
        return this.request('/bff/v1/system/bootstrap', { method: 'POST', body: JSON.stringify({ username }) });
    },

    login(accessToken, fingerprint, opts = {}) {
        const body = { access_token: accessToken, fingerprint };
        if (opts && opts.remember_me) body.remember_me = true;
        if (opts && opts.device_name) body.device_name = String(opts.device_name);
        return this.request('/bff/v1/auth/login', {
            method: 'POST',
            body: JSON.stringify(body)
        });
    },

    deviceLogin(fingerprint) {
        return this.request('/bff/v1/auth/device_login', {
            method: 'POST',
            body: JSON.stringify({ fingerprint })
        });
    },

    logout() {
        return this.request('/bff/v1/auth/logout', { method: 'POST', body: '{}' });
    },

    getProfile() {
        return this.request('/bff/v1/me/profile');
    },

    listMyDevices() {
        return this.request('/bff/v1/me/devices');
    },

    patchDeviceAlias(sessionId, alias) {
        return this.request(`/bff/v1/me/devices/${encodeURIComponent(sessionId)}/alias`, {
            method: 'PATCH',
            body: JSON.stringify({ alias })
        });
    },

    deleteDevice(sessionId) {
        return this.request(`/bff/v1/me/devices/${encodeURIComponent(sessionId)}`, { method: 'DELETE' });
    },

    revokeAllSessions() {
        return this.request('/bff/v1/me/sessions/revoke_all', { method: 'POST', body: '{}' });
    },

    passkeyStatus() {
        return this.request('/bff/v1/me/security/passkeys/status').then((r) => r.status || null);
    },

    listPasskeys() {
        return this.request('/bff/v1/me/security/passkeys').then((r) => r.passkeys || []);
    },

    deletePasskey(id) {
        return this.request(`/bff/v1/me/security/passkeys/${encodeURIComponent(id)}`, { method: 'DELETE' });
    },

    beginPasskeyRegistration(deviceName) {
        return this.request('/bff/v1/me/security/passkeys/registration/begin', {
            method: 'POST',
            body: JSON.stringify({ device_name: deviceName || '' })
        });
    },

    finishPasskeyRegistration(payload) {
        return this.request('/bff/v1/me/security/passkeys/registration/finish', {
            method: 'POST',
            body: JSON.stringify(payload)
        });
    },

    beginPasskeyVerification() {
        return this.request('/bff/v1/me/security/passkeys/verification/begin', { method: 'POST', body: '{}' });
    },

    finishPasskeyVerification(payload) {
        return this.request('/bff/v1/me/security/passkeys/verification/finish', {
            method: 'POST',
            body: JSON.stringify(payload)
        });
    },

    listDeviceTokens() {
        return this.request('/bff/v1/me/security/device_tokens').then((r) => r.device_tokens || []);
    },

    createDeviceToken(deviceName) {
        return this.request('/bff/v1/me/security/device_tokens', {
            method: 'POST',
            body: JSON.stringify({ device_name: deviceName || '' })
        });
    },

    revokeDeviceToken(id) {
        return this.request(`/bff/v1/me/security/device_tokens/${encodeURIComponent(id)}`, { method: 'DELETE' });
    },

    revokeAllDeviceTokens() {
        return this.request('/bff/v1/me/security/device_tokens/revoke_all', { method: 'POST', body: '{}' });
    },

    adminListUsers() {
        return this.request('/bff/v1/admin/users');
    },

    adminCreateUser(username, role) {
        return this.request('/bff/v1/admin/users', { method: 'POST', body: JSON.stringify({ username, role }) });
    },

    adminResetUserToken(userId) {
        return this.request(`/bff/v1/admin/users/${encodeURIComponent(userId)}/reset_token`, { method: 'POST', body: '{}' });
    },

    adminListAPIKeys() {
        return this.request('/bff/v1/admin/api_keys');
    },

    adminCreateAPIKey(name, scope, expiresAt) {
        const body = { name, scope };
        if (expiresAt) body.expires_at = expiresAt;
        return this.request('/bff/v1/admin/api_keys', { method: 'POST', body: JSON.stringify(body) });
    },

    adminRevokeAPIKey(id) {
        return this.request(`/bff/v1/admin/api_keys/${encodeURIComponent(id)}/revoke`, { method: 'POST', body: '{}' });
    },

    adminAPIKeyUsage(days) {
        return this.request(`/bff/v1/admin/api_keys/usage?days=${encodeURIComponent(String(days || 7))}`);
    },

    adminListAllDevices() {
        return this.request('/bff/v1/admin/devices');
    },

    adminConfigSnapshot() {
        return this.request('/bff/v1/admin/config/snapshot');
    },

    getDocuments: () => apiClient.request('/bff/v1/documents').then((r) => r.documents || []),
    getDocument: (id) => apiClient.request(`/bff/v1/documents/${id}`),
    getDocumentChapters: (id) => apiClient.request(`/bff/v1/documents/${id}/chapters`).then((r) => r.chapters || []),
    createDocument: (data) => apiClient.request('/bff/v1/documents', { method: 'POST', body: JSON.stringify(data) }),
    promoteDocument: (id) => apiClient.request(`/bff/v1/documents/${id}/promote`, { method: 'POST', body: '{}' }),

    progressiveQuery: (question, options = {}) => {
        const max = options.max_results != null ? options.max_results : 15;
        const payload = { question, max_results: max };
        let path = '/bff/v1/query/progressive';
        if (options.trace) {
            path += '?debug_trace=1';
        }
        return apiClient.request(path, {
            method: 'POST',
            body: JSON.stringify(payload)
        });
    },

    getTags: (opts = {}) => {
        const q = new URLSearchParams();
        if (opts.topic_ids && opts.topic_ids.length) {
            q.set('topic_ids', opts.topic_ids.filter(Boolean).join(','));
        }
        if (opts.max_results != null && opts.max_results > 0) {
            q.set('max_results', String(opts.max_results));
        }
        const suffix = q.toString() ? `?${q.toString()}` : '';
        return apiClient.request(`/bff/v1/tags${suffix}`).then((r) => r.tags || []);
    },
    getTopics: () => apiClient.request('/bff/v1/topics').then((r) => r.topics || []),
    triggerTopicRegroup: () => apiClient.request('/bff/v1/topics/regroup', { method: 'POST' }),

    getMonitoring: () => apiClient.request('/bff/v1/monitoring'),
    getMetricsText: () => apiClient.requestText('/metrics'),

    /** Cold hybrid search: `q` comma-separated keywords; `max_results` 1–500 (server clamps). */
    getColdChapterHits(q, maxResults) {
        const params = new URLSearchParams();
        params.set('q', String(q || '').trim());
        if (maxResults != null && maxResults > 0) {
            params.set('max_results', String(Math.min(500, maxResults)));
        }
        return apiClient.request(`/bff/v1/cold/chapter_hits?${params.toString()}`);
    },

    /** Legacy alias (tiersum_bak UI) for cold probe. */
    getColdDocSource(q, maxResults) {
        const params = new URLSearchParams();
        params.set('q', String(q || '').trim());
        if (maxResults != null && maxResults > 0) {
            params.set('max_results', String(Math.min(500, maxResults)));
        }
        return apiClient.request(`/bff/v1/cold/doc_source?${params.toString()}`);
    },

    listTraces: (params = {}) => {
        const q = new URLSearchParams();
        if (params.limit != null) q.set('limit', String(params.limit));
        if (params.offset != null) q.set('offset', String(params.offset));
        const s = q.toString();
        return apiClient.request('/bff/v1/traces' + (s ? `?${s}` : ''));
    },

    getTrace: (traceId) => apiClient.request(`/bff/v1/traces/${encodeURIComponent(traceId)}`)
};
