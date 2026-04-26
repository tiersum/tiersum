import { apiClient } from '../api_client.js';

export const LoginPage = {
    data() {
        return {
            accessToken: '',
            rememberMe: false,
            deviceName: '',
            loading: false,
            err: ''
        };
    },
    async mounted() {
        // If a persistent device token cookie exists, try passwordless re-login first.
        // Only try once; if it fails the user stays on the login form.
        if (this.$route.query.auto !== '0') {
            await this.tryDeviceLogin();
        }
    },
    methods: {
        fingerprint() {
            const tz = Intl.DateTimeFormat().resolvedOptions().timeZone || '';
            return { timezone: tz, client_signal: '' };
        },
        async tryDeviceLogin() {
            this.err = '';
            this.loading = true;
            try {
                await apiClient.deviceLogin(this.fingerprint());
                this.$router.replace('/search');
            } catch {
                // Expected when no device cookie / invalid token.
            } finally {
                this.loading = false;
            }
        },
        async submit() {
            this.err = '';
            this.loading = true;
            try {
                await apiClient.login(this.accessToken.trim(), this.fingerprint(), {
                    remember_me: this.rememberMe,
                });
                this.$router.replace('/search');
            } catch (e) {
                this.err = e.message || String(e);
            } finally {
                this.loading = false;
            }
        }
    },
    template: `
        <div class="max-w-lg mx-auto px-4 py-16">
            <h1 class="text-2xl font-bold text-slate-100 mb-2">{{ $t('loginTitle') }}</h1>
            <p class="text-slate-400 text-sm mb-6">{{ $t('loginDesc') }}</p>
            <label class="form-control w-full">
                <span class="label-text text-slate-300">{{ $t('loginToken') }}</span>
                <textarea v-model="accessToken" class="textarea textarea-bordered bg-slate-900 border-slate-700 text-slate-100 font-mono text-sm h-28" :placeholder="$t('loginTokenPlaceholder')"></textarea>
            </label>
            <label class="label cursor-pointer justify-start gap-3 mt-3">
                <input type="checkbox" v-model="rememberMe" class="checkbox checkbox-sm" />
                <span class="label-text text-slate-300">{{ $t('loginRememberMe') }}</span>
            </label>
            <label v-if="rememberMe" class="form-control w-full mt-3">
                <span class="label-text text-slate-300">{{ $t('loginDeviceLabel') }}</span>
                <input v-model="deviceName" class="input input-bordered bg-slate-900 border-slate-700 text-slate-100" :placeholder="$t('loginDevicePlaceholder')" />
            </label>
            <p v-if="err" class="text-sm text-red-400 mt-2">{{ err }}</p>
            <button class="btn btn-primary w-full mt-4" :disabled="loading || !accessToken.trim()" @click="submit">
                {{ loading ? $t('loginVerifying') : $t('loginVerify') }}
            </button>
            <div class="divider text-slate-600">{{ $t('loginOr') }}</div>
            <button class="btn btn-outline w-full" :disabled="loading" @click="tryDeviceLogin">
                {{ $t('loginDeviceSignIn') }}
            </button>
        </div>
    `
};
