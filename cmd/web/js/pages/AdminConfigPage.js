import { apiClient } from '../api_client.js';

/** Read-only redacted effective config for admin role (`GET /bff/v1/admin/config/snapshot`). */
export const AdminConfigPage = {
    data() {
        return {
            configSnapshot: null,
            configMeta: null,
            configLoading: false,
            configErr: ''
        };
    },
    async mounted() {
        await this.loadSnapshot();
    },
    methods: {
        async loadSnapshot() {
            this.configLoading = true;
            this.configErr = '';
            try {
                const r = await apiClient.adminConfigSnapshot();
                this.configSnapshot = r.snapshot || {};
                this.configMeta = { source: r.source, generated_at: r.generated_at };
            } catch (e) {
                this.configErr = e.message || String(e);
            } finally {
                this.configLoading = false;
            }
        }
    },
    template: `
        <div class="max-w-5xl mx-auto px-4 py-8">
            <h1 class="text-2xl font-bold text-slate-100 mb-2">{{ $t('configTitle') }}</h1>
            <p class="text-slate-500 text-sm mb-6">{{ $t('configDesc') }}</p>
            <p class="text-slate-400 text-sm mb-4">
                {{ $t('configReadOnly') }}
            </p>
            <div class="flex gap-2 mb-4">
                <button type="button" class="btn btn-sm btn-outline border-slate-600" :disabled="configLoading" @click="loadSnapshot">{{ $t('refresh') }}</button>
            </div>
            <p v-if="configErr" class="text-sm text-red-400">{{ configErr }}</p>
            <p v-else-if="configLoading" class="text-slate-500 text-sm">{{ $t('loading') }}</p>
            <div v-else-if="configMeta" class="rounded-lg border border-slate-800 bg-slate-950/50 p-3">
                <p class="text-xs text-slate-500 mb-2">{{ $t('configSource') }}: {{ configMeta.source }} · {{ $t('configGenerated') }}: {{ configMeta.generated_at }}</p>
                <pre class="text-[11px] leading-relaxed text-slate-300 overflow-x-auto max-h-[70vh] overflow-y-auto whitespace-pre-wrap break-words">{{ JSON.stringify(configSnapshot, null, 2) }}</pre>
            </div>
        </div>
    `
};
