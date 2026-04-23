import { apiClient } from '../api_client.js';

export const InitPage = {
    data() {
        return {
            username: 'admin',
            loading: false,
            done: false,
            result: null,
            err: ''
        };
    },
    methods: {
        async submit() {
            this.err = '';
            this.loading = true;
            try {
                const r = await apiClient.bootstrap(this.username.trim());
                this.result = r;
                this.done = true;
            } catch (e) {
                this.err = e.message || String(e);
            } finally {
                this.loading = false;
            }
        }
    },
    template: `
        <div class="max-w-lg mx-auto px-4 py-16">
            <h1 class="text-2xl font-bold text-slate-100 mb-2">{{ $t('initTitle') }}</h1>
            <p class="text-slate-400 text-sm mb-6">{{ $t('initDesc') }}</p>
            <div v-if="!done" class="space-y-4">
                <label class="form-control w-full">
                    <span class="label-text text-slate-300">{{ $t('initUsername') }}</span>
                    <input v-model="username" type="text" class="input input-bordered bg-slate-900 border-slate-700 text-slate-100" autocomplete="username" />
                </label>
                <p v-if="err" class="text-sm text-red-400">{{ err }}</p>
                <button class="btn btn-primary w-full" :disabled="loading || !username.trim()" @click="submit">
                    {{ loading ? $t('initWorking') : $t('initButton') }}
                </button>
            </div>
            <div v-else class="space-y-4 rounded-lg border border-red-900/60 bg-red-950/30 p-4">
                <p class="text-red-300 font-semibold">{{ $t('initCopySecrets') }}</p>
                <div class="text-sm text-slate-300 space-y-2">
                    <div><span class="text-slate-500">{{ $t('initAdminToken') }}</span>
                        <pre class="mt-1 p-2 bg-slate-900 rounded text-emerald-300 whitespace-pre-wrap break-all">{{ result.admin_access_token }}</pre>
                    </div>
                    <div><span class="text-slate-500">{{ $t('initAPIKey') }}</span>
                        <pre class="mt-1 p-2 bg-slate-900 rounded text-emerald-300 whitespace-pre-wrap break-all">{{ result.initial_api_key }}</pre>
                    </div>
                </div>
                <router-link to="/login" class="btn btn-secondary w-full">{{ $t('initContinueLogin') }}</router-link>
            </div>
        </div>
    `
};
