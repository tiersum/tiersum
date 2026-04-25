import { apiClient, isBrowserAdminRole } from '../api_client.js';

export const SettingsPage = {
    data() {
        return {
            devices: [],
            passkeys: [],
            passkeyStatus: null,
            deviceTokens: [],
            isAdmin: false,
            loading: true,
            err: '',
            aliasEdits: {},
            newDeviceTokenName: ''
        };
    },
    async mounted() {
        await this.load();
    },
    methods: {
        isBrowserAdminRole,
        async load() {
            this.loading = true;
            this.err = '';
            try {
                const profile = await apiClient.getProfile();
                this.isAdmin = isBrowserAdminRole(profile.role);
                const r = this.isAdmin
                    ? await apiClient.adminListAllDevices()
                    : await apiClient.listMyDevices();
                this.devices = r.devices || [];
                await this.loadSecurity();
            } catch (e) {
                this.err = e.message || String(e);
            } finally {
                this.loading = false;
            }
        },
        async loadSecurity() {
            try {
                this.passkeyStatus = await apiClient.passkeyStatus();
                this.passkeys = await apiClient.listPasskeys();
                this.deviceTokens = await apiClient.listDeviceTokens();
            } catch {
                // Non-fatal; keep page usable.
                this.passkeyStatus = null;
                this.passkeys = [];
                this.deviceTokens = [];
            }
        },
        toJSONBytes(buf) {
            return Array.from(new Uint8Array(buf));
        },
        async registerPasskey() {
            const name = window.prompt(this.$t('settingsLabelOptional'), 'passkey') || '';
            try {
                const begin = await apiClient.beginPasskeyRegistration(name);
                const opts = begin && begin.publicKey ? begin.publicKey : null;
                if (!opts || !window.PublicKeyCredential) {
                    alert('WebAuthn is not available in this browser/context (requires HTTPS + Origin).');
                    return;
                }
                const cred = await navigator.credentials.create({ publicKey: opts });
                const transports = cred.response.getTransports ? cred.response.getTransports() : [];
                const payload = {
                    session_id: begin.session_id,
                    device_name: (begin.device_name || name || '').trim(),
                    credential: {
                        id: cred.id,
                        rawId: this.toJSONBytes(cred.rawId),
                        type: cred.type,
                        response: {
                            clientDataJSON: this.toJSONBytes(cred.response.clientDataJSON),
                            attestationObject: this.toJSONBytes(cred.response.attestationObject)
                        },
                        transports
                    }
                };
                await apiClient.finishPasskeyRegistration(payload);
                await this.loadSecurity();
                alert('Passkey registered.');
            } catch (e) {
                alert(e.message || String(e));
            }
        },
        async verifyPasskey() {
            try {
                const begin = await apiClient.beginPasskeyVerification();
                const opts = begin && begin.publicKey ? begin.publicKey : null;
                if (!opts || !window.PublicKeyCredential) {
                    alert('WebAuthn is not available in this browser/context (requires HTTPS + Origin).');
                    return;
                }
                const assertion = await navigator.credentials.get({ publicKey: opts });
                const payload = {
                    session_id: begin.session_id,
                    credential: {
                        id: assertion.id,
                        rawId: this.toJSONBytes(assertion.rawId),
                        type: assertion.type,
                        response: {
                            clientDataJSON: this.toJSONBytes(assertion.response.clientDataJSON),
                            authenticatorData: this.toJSONBytes(assertion.response.authenticatorData),
                            signature: this.toJSONBytes(assertion.response.signature),
                            userHandle: assertion.response.userHandle
                                ? this.toJSONBytes(assertion.response.userHandle)
                                : null
                        }
                    }
                };
                await apiClient.finishPasskeyVerification(payload);
                await this.loadSecurity();
                alert('Passkey verified for this browser session.');
            } catch (e) {
                alert(e.message || String(e));
            }
        },
        async revokePasskey(id) {
            if (!confirm(this.$t('remove') + ' ' + this.$t('settingsPasskeys') + '?')) return;
            try {
                await apiClient.deletePasskey(id);
                await this.loadSecurity();
            } catch (e) {
                alert(e.message || String(e));
            }
        },
        async createDeviceToken() {
            try {
                await apiClient.createDeviceToken(this.newDeviceTokenName);
                this.newDeviceTokenName = '';
                await this.loadSecurity();
                alert('Device token cookie set for this browser.');
            } catch (e) {
                alert(e.message || String(e));
            }
        },
        async revokeDeviceToken(id) {
            if (!confirm(this.$t('revoke') + ' ' + this.$t('settingsDeviceTokens') + '?')) return;
            try {
                await apiClient.revokeDeviceToken(id);
                await this.loadSecurity();
            } catch (e) {
                alert(e.message || String(e));
            }
        },
        async revokeAllDeviceTokens() {
            if (!confirm(this.$t('revokeAll') + ' ' + this.$t('settingsDeviceTokens') + '?')) return;
            try {
                await apiClient.revokeAllDeviceTokens();
                await this.loadSecurity();
            } catch (e) {
                alert(e.message || String(e));
            }
        },
        async saveAlias(id) {
            const alias = this.aliasEdits[id] ?? '';
            try {
                await apiClient.patchDeviceAlias(id, alias);
                await this.load();
            } catch (e) {
                alert(e.message || String(e));
            }
        },
        async revoke(id) {
            if (!confirm(this.$t('settingsSignOut') + '?')) return;
            try {
                await apiClient.deleteDevice(id);
                await this.load();
            } catch (e) {
                alert(e.message || String(e));
            }
        },
        async revokeAll() {
            const msg = this.isAdmin
                ? this.$t('settingsSignOutAll') + ' (' + this.$t('adminUser') + ' unchanged)?'
                : this.$t('settingsSignOutAll') + '?';
            if (!confirm(msg)) return;
            try {
                await apiClient.revokeAllSessions();
                this.$router.replace('/login');
            } catch (e) {
                alert(e.message || String(e));
            }
        }
    },
    template: `
        <div class="max-w-[1600px] mx-auto px-4 sm:px-6 lg:px-8 py-8">
            <h1 class="text-2xl font-bold text-slate-100 mb-4">{{ $t('settingsTitle') }}</h1>
            <p v-if="isAdmin && !loading && !err" class="text-slate-400 text-sm mb-2">{{ $t('settingsAdminNote') }}</p>
            <p v-if="loading" class="text-slate-400">{{ $t('loading') }}</p>
            <p v-else-if="err" class="text-red-400">{{ err }}</p>
            <div v-else class="space-y-10">
                <section class="rounded-lg border border-slate-800 bg-slate-900/40 p-4">
                    <div class="flex items-start justify-between gap-3">
                        <div>
                            <h2 class="text-lg font-semibold text-slate-100">{{ $t('settingsPasskeys') }}</h2>
                            <p class="text-slate-400 text-sm mt-1">
                                {{ $t('settingsPasskeysDesc') }}
                            </p>
                            <p v-if="passkeyStatus" class="text-slate-500 text-xs mt-2">
                                has_any={{ passkeyStatus.has_any }} · verified={{ !!passkeyStatus.verified_at }} · admin_required={{ passkeyStatus.required_for_admin }}
                            </p>
                        </div>
                        <div class="flex flex-col gap-2">
                            <button class="btn btn-sm btn-primary" @click="registerPasskey">{{ $t('settingsRegisterPasskey') }}</button>
                            <button class="btn btn-sm btn-outline border-slate-600" @click="verifyPasskey">{{ $t('settingsVerifyPasskey') }}</button>
                        </div>
                    </div>
                    <div class="mt-4 space-y-2">
                        <div v-for="p in passkeys" :key="p.id" class="rounded-md border border-slate-800 bg-slate-950/40 p-3 flex items-center justify-between gap-3">
                            <div>
                                <div class="text-slate-200 text-sm">{{ p.device_name }}</div>
                                <div class="text-slate-500 text-xs">id {{ p.id.slice(0, 8) }}…</div>
                            </div>
                            <button class="btn btn-xs btn-outline border-slate-700" @click="revokePasskey(p.id)">{{ $t('remove') }}</button>
                        </div>
                        <p v-if="!passkeys.length" class="text-slate-500 text-sm">{{ $t('settingsNoPasskeys') }}</p>
                    </div>
                </section>

                <section class="rounded-lg border border-slate-800 bg-slate-900/40 p-4">
                    <h2 class="text-lg font-semibold text-slate-100 mb-2">{{ $t('settingsDeviceTokens') }}</h2>
                    <p class="text-slate-400 text-sm mb-3">{{ $t('settingsDeviceTokensDesc') }}</p>
                    <div class="flex gap-2 items-center">
                        <input v-model="newDeviceTokenName" class="input input-bordered input-sm bg-slate-950 border-slate-700 flex-1" :placeholder="$t('settingsLabelOptional')" />
                        <button class="btn btn-sm btn-primary" @click="createDeviceToken">{{ $t('settingsCreateCookie') }}</button>
                        <button class="btn btn-sm btn-outline border-slate-600" @click="revokeAllDeviceTokens">{{ $t('settingsRevokeAll') }}</button>
                    </div>
                    <div class="mt-4 space-y-2">
                        <div v-for="t in deviceTokens" :key="t.id" class="rounded-md border border-slate-800 bg-slate-950/40 p-3 flex items-center justify-between gap-3">
                            <div>
                                <div class="text-slate-200 text-sm">{{ t.device_name }}</div>
                                <div class="text-slate-500 text-xs">expires {{ new Date(t.expires_at).toLocaleString() }}</div>
                            </div>
                            <button class="btn btn-xs btn-outline border-slate-700" @click="revokeDeviceToken(t.id)">{{ $t('revoke') }}</button>
                        </div>
                        <p v-if="!deviceTokens.length" class="text-slate-500 text-sm">{{ $t('settingsNoTokens') }}</p>
                    </div>
                </section>

                <section>
                    <h2 class="text-xl font-bold text-slate-100 mb-4">{{ $t('settingsBrowserSessions') }}</h2>
                    <div class="flex justify-end">
                        <button class="btn btn-outline btn-sm border-red-800 text-red-300" @click="revokeAll">{{ $t('settingsSignOutAll') }}</button>
                    </div>
                    <div v-for="d in devices" :key="d.session_id" class="rounded-lg border border-slate-800 bg-slate-900/50 p-4">
                        <div v-if="isAdmin" class="text-emerald-400/90 text-xs font-medium mb-1">{{ d.username }}</div>
                        <div class="text-slate-300 text-sm">{{ $t('settingsSession') }} {{ d.session_id.slice(0, 8) }}…</div>
                        <div class="text-slate-500 text-xs mt-1">{{ $t('settingsIPPrefix') }} {{ d.ip_prefix }} · {{ $t('settingsLastSeen') }} {{ new Date(d.last_seen_at).toLocaleString() }}</div>
                        <div class="flex gap-2 mt-3 items-center">
                            <input v-model="aliasEdits[d.session_id]" :placeholder="d.device_alias || $t('settingsDeviceAlias')" class="input input-bordered input-sm bg-slate-950 border-slate-700 flex-1" />
                            <button class="btn btn-sm btn-ghost" @click="saveAlias(d.session_id)">{{ $t('settingsSaveAlias') }}</button>
                            <button class="btn btn-sm btn-outline border-slate-600" @click="revoke(d.session_id)">{{ $t('settingsSignOut') }}</button>
                        </div>
                    </div>
                    <p v-if="!devices.length" class="text-slate-500">{{ $t('settingsNoSessions') }}</p>
                </section>
            </div>
        </div>
    `
};
