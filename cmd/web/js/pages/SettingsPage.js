import { apiClient, isBrowserAdminRole } from '../api_client.js';

export const SettingsPage = {
    data() {
        return {
            devices: [],
            isAdmin: false,
            loading: true,
            err: '',
            aliasEdits: {}
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
            } catch (e) {
                this.err = e.message || String(e);
            } finally {
                this.loading = false;
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
            if (!confirm('Sign out this device?')) return;
            try {
                await apiClient.deleteDevice(id);
                await this.load();
            } catch (e) {
                alert(e.message || String(e));
            }
        },
        async revokeAll() {
            const msg = this.isAdmin
                ? 'Sign out all devices registered to your account (other users are unchanged)?'
                : 'Sign out all devices (including this browser)?';
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
        <div class="max-w-3xl mx-auto px-4 py-8">
            <h1 class="text-2xl font-bold text-slate-100 mb-4">Devices</h1>
            <p v-if="isAdmin && !loading && !err" class="text-slate-400 text-sm mb-2">Administrator view: all browser sessions for every user.</p>
            <p v-if="loading" class="text-slate-400">Loading…</p>
            <p v-else-if="err" class="text-red-400">{{ err }}</p>
            <div v-else class="space-y-4">
                <div class="flex justify-end">
                    <button class="btn btn-outline btn-sm border-red-800 text-red-300" @click="revokeAll">Sign out all my devices</button>
                </div>
                <div v-for="d in devices" :key="d.session_id" class="rounded-lg border border-slate-800 bg-slate-900/50 p-4">
                    <div v-if="isAdmin" class="text-emerald-400/90 text-xs font-medium mb-1">{{ d.username }}</div>
                    <div class="text-slate-300 text-sm">Session {{ d.session_id.slice(0, 8) }}…</div>
                    <div class="text-slate-500 text-xs mt-1">IP prefix {{ d.ip_prefix }} · last seen {{ new Date(d.last_seen_at).toLocaleString() }}</div>
                    <div class="flex gap-2 mt-3 items-center">
                        <input v-model="aliasEdits[d.session_id]" :placeholder="d.device_alias || 'Device alias'" class="input input-bordered input-sm bg-slate-950 border-slate-700 flex-1" />
                        <button class="btn btn-sm btn-ghost" @click="saveAlias(d.session_id)">Save alias</button>
                        <button class="btn btn-sm btn-outline border-slate-600" @click="revoke(d.session_id)">Sign out</button>
                    </div>
                </div>
                <p v-if="!devices.length" class="text-slate-500">No active sessions.</p>
            </div>
        </div>
    `
};
