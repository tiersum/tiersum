import { apiClient } from '../api_client.js';

export const LoginPage = {
    data() {
        return {
            accessToken: '',
            loading: false,
            err: ''
        };
    },
    methods: {
        async submit() {
            this.err = '';
            this.loading = true;
            try {
                const tz = Intl.DateTimeFormat().resolvedOptions().timeZone || '';
                await apiClient.login(this.accessToken.trim(), { timezone: tz, client_signal: '' });
                this.$router.replace('/');
            } catch (e) {
                this.err = e.message || String(e);
            } finally {
                this.loading = false;
            }
        }
    },
    template: `
        <div class="max-w-lg mx-auto px-4 py-16">
            <h1 class="text-2xl font-bold text-slate-100 mb-2">Sign in</h1>
            <p class="text-slate-400 text-sm mb-6">Paste the access token issued by your administrator.</p>
            <label class="form-control w-full">
                <span class="label-text text-slate-300">Access token</span>
                <textarea v-model="accessToken" class="textarea textarea-bordered bg-slate-900 border-slate-700 text-slate-100 font-mono text-sm h-28" placeholder="ts_u_…"></textarea>
            </label>
            <p v-if="err" class="text-sm text-red-400 mt-2">{{ err }}</p>
            <button class="btn btn-primary w-full mt-4" :disabled="loading || !accessToken.trim()" @click="submit">
                {{ loading ? 'Verifying…' : 'Verify and bind device' }}
            </button>
        </div>
    `
};
