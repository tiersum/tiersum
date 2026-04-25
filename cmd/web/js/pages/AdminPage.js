import { apiClient } from '../api_client.js';

export const AdminPage = {
    data() {
        return {
            tab: 'users',
            users: [],
            keys: [],
            allDevices: [],
            usage: null,
            newUser: { username: '', role: 'user' },
            newKey: { name: '', scope: 'read' },
            lastSecret: '',
            err: '',
            loading: false,
            aliasEdits: {}
        };
    },
    async mounted() {
        await this.refreshAll();
    },
    methods: {
        async refreshAll() {
            this.err = '';
            this.loading = true;
            try {
                const [u, k, d, us] = await Promise.all([
                    apiClient.adminListUsers(),
                    apiClient.adminListAPIKeys(),
                    apiClient.adminListAllDevices(),
                    apiClient.adminAPIKeyUsage(7)
                ]);
                this.users = u.users || [];
                this.keys = k.api_keys || [];
                this.allDevices = d.devices || [];
                this.usage = us;
            } catch (e) {
                this.err = e.message || String(e);
            } finally {
                this.loading = false;
            }
        },
        async createUser() {
            this.lastSecret = '';
            try {
                const r = await apiClient.adminCreateUser(this.newUser.username.trim(), this.newUser.role);
                this.lastSecret = r.access_token;
                this.newUser.username = '';
                await this.refreshAll();
            } catch (e) {
                alert(e.message || String(e));
            }
        },
        async resetToken(id) {
            if (!confirm(this.$t('adminResetConfirm'))) return;
            try {
                const r = await apiClient.adminResetUserToken(id);
                this.lastSecret = r.access_token;
                await this.refreshAll();
            } catch (e) {
                alert(e.message || String(e));
            }
        },
        async createKey() {
            this.lastSecret = '';
            try {
                const r = await apiClient.adminCreateAPIKey(this.newKey.name.trim(), this.newKey.scope, null);
                this.lastSecret = r.api_key;
                this.newKey.name = '';
                await this.refreshAll();
            } catch (e) {
                alert(e.message || String(e));
            }
        },
        async revokeKey(id) {
            if (!confirm(this.$t('adminRevokeConfirm'))) return;
            try {
                await apiClient.adminRevokeAPIKey(id);
                await this.refreshAll();
            } catch (e) {
                alert(e.message || String(e));
            }
        },
        async adminSaveAlias(sessionId) {
            const alias = this.aliasEdits[sessionId] ?? '';
            try {
                await apiClient.patchDeviceAlias(sessionId, alias);
                await this.refreshAll();
            } catch (e) {
                alert(e.message || String(e));
            }
        },
        async adminRevokeDevice(sessionId) {
            if (!confirm(this.$t('adminSignOutConfirm'))) return;
            try {
                await apiClient.deleteDevice(sessionId);
                await this.refreshAll();
            } catch (e) {
                alert(e.message || String(e));
            }
        }
    },
    template: `
        <div class="max-w-[1600px] mx-auto px-4 sm:px-6 lg:px-8 py-8">
            <h1 class="text-2xl font-bold text-slate-100 mb-2">{{ $t('adminTitle') }}</h1>
            <p class="text-slate-500 text-sm mb-6">{{ $t('adminDesc') }}</p>
            <p v-if="loading" class="text-slate-400">{{ $t('loading') }}</p>
            <p v-else-if="err" class="text-red-400">{{ err }}</p>
            <div v-else>
                <div v-if="lastSecret" class="mb-6 rounded-lg border border-amber-800/60 bg-amber-950/30 p-4">
                    <p class="text-amber-200 font-semibold text-sm">{{ $t('adminNewSecret') }}</p>
                    <pre class="mt-2 text-xs text-emerald-300 whitespace-pre-wrap break-all">{{ lastSecret }}</pre>
                </div>
                <div class="tabs tabs-boxed bg-slate-900/80 mb-4 flex flex-wrap gap-y-1">
                    <a :class="['tab', tab==='users' && 'tab-active']" @click.prevent="tab='users'">{{ $t('adminTabUsers') }}</a>
                    <a :class="['tab', tab==='devices' && 'tab-active']" @click.prevent="tab='devices'">{{ $t('adminTabDevices') }}</a>
                    <a :class="['tab', tab==='keys' && 'tab-active']" @click.prevent="tab='keys'">{{ $t('adminTabKeys') }}</a>
                </div>
                <div v-show="tab==='users'" class="space-y-4">
                    <div class="flex flex-wrap gap-2 items-end">
                        <input v-model="newUser.username" :placeholder="$t('adminUser')" class="input input-bordered bg-slate-900 border-slate-700" />
                        <select v-model="newUser.role" class="select select-bordered bg-slate-900 border-slate-700">
                            <option value="user">user</option>
                            <option value="viewer">viewer</option>
                            <option value="admin">admin</option>
                        </select>
                        <button class="btn btn-primary btn-sm" @click="createUser">{{ $t('add') }} {{ $t('adminUser') }}</button>
                    </div>
                    <table class="table table-zebra text-sm">
                        <thead><tr><th>{{ $t('adminUser') }}</th><th>{{ $t('adminRole') }}</th><th>{{ $t('adminActions') }}</th></tr></thead>
                        <tbody>
                            <tr v-for="u in users" :key="u.id">
                                <td>{{ u.username }}</td>
                                <td>{{ u.role }}</td>
                                <td><button class="btn btn-xs btn-outline" @click="resetToken(u.id)">{{ $t('adminResetToken') }}</button></td>
                            </tr>
                        </tbody>
                    </table>
                </div>
                <div v-show="tab==='devices'" class="space-y-4">
                    <p class="text-slate-400 text-sm">{{ $t('adminDevicesDesc') }}</p>
                    <table class="table table-zebra text-sm">
                        <thead><tr><th>{{ $t('adminUser') }}</th><th>{{ $t('adminAlias') }}</th><th>{{ $t('adminIPPrefix') }}</th><th>{{ $t('adminLastSeen') }}</th><th>{{ $t('adminExpires') }}</th><th></th></tr></thead>
                        <tbody>
                            <tr v-for="d in allDevices" :key="d.session_id">
                                <td>{{ d.username }}</td>
                                <td>
                                    <div class="flex gap-1 items-center max-w-xs">
                                        <input v-model="aliasEdits[d.session_id]" :placeholder="d.device_alias || $t('adminAlias')" class="input input-bordered input-xs bg-slate-950 border-slate-700 flex-1 min-w-0" />
                                        <button class="btn btn-xs btn-ghost shrink-0" @click="adminSaveAlias(d.session_id)">{{ $t('save') }}</button>
                                    </div>
                                </td>
                                <td>{{ d.ip_prefix }}</td>
                                <td>{{ new Date(d.last_seen_at).toLocaleString() }}</td>
                                <td>{{ new Date(d.expires_at).toLocaleString() }}</td>
                                <td><button class="btn btn-xs btn-outline border-red-800 text-red-300" @click="adminRevokeDevice(d.session_id)">{{ $t('adminSignOut') }}</button></td>
                            </tr>
                        </tbody>
                    </table>
                    <p v-if="!allDevices.length" class="text-slate-500 text-sm">{{ $t('adminNoSessions') }}</p>
                </div>
                <div v-show="tab==='keys'" class="space-y-4">
                    <div class="flex flex-wrap gap-2 items-end">
                        <input v-model="newKey.name" :placeholder="$t('adminKeyName')" class="input input-bordered bg-slate-900 border-slate-700" />
                        <select v-model="newKey.scope" class="select select-bordered bg-slate-900 border-slate-700">
                            <option value="read">read</option>
                            <option value="write">write</option>
                            <option value="admin">admin</option>
                        </select>
                        <button class="btn btn-primary btn-sm" @click="createKey">{{ $t('adminCreateKey') }}</button>
                    </div>
                    <table class="table table-zebra text-sm">
                        <thead><tr><th>{{ $t('adminName') }}</th><th>{{ $t('adminScope') }}</th><th>{{ $t('adminStatus') }}</th><th></th></tr></thead>
                        <tbody>
                            <tr v-for="k in keys" :key="k.id">
                                <td>{{ k.name }}</td>
                                <td>{{ k.scope }}</td>
                                <td>{{ k.revoked_at ? $t('adminRevoked') : $t('adminActive') }}</td>
                                <td><button v-if="!k.revoked_at" class="btn btn-xs btn-outline border-red-800 text-red-300" @click="revokeKey(k.id)">{{ $t('adminRevoke') }}</button></td>
                            </tr>
                        </tbody>
                    </table>
                    <p class="text-slate-500 text-xs">{{ $t('adminUsage') }} {{ JSON.stringify(usage?.counts_by_key_id || {}) }}</p>
                </div>
            </div>
        </div>
    `
};
