import { apiClient, isBrowserAdminRole } from '../api_client.js';

export const AppHeader = {
    data() {
        return { me: null };
    },
    computed: {
        securityMenuActive() {
            const p = this.$route?.path || '';
            return p === '/settings' || p.startsWith('/admin');
        }
    },
    async mounted() {
        try {
            const st = await apiClient.getSystemStatus();
            if (!st.initialized) {
                this.me = null;
                return;
            }
            this.me = await apiClient.getProfile();
        } catch {
            this.me = null;
        }
    },
    methods: {
        isBrowserAdminRole,
        async logout() {
            try {
                await apiClient.logout();
            } catch {
                /* ignore */
            }
            this.me = null;
            window.location.assign('/login');
        }
    },
    template: `
        <header class="border-b border-slate-800 bg-slate-950/80 backdrop-blur-md sticky top-0 z-50">
            <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center justify-between">
                <router-link to="/" class="flex items-center gap-2 hover:opacity-80 transition-opacity">
                    <svg class="w-7 h-7 text-blue-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z"/>
                    </svg>
                    <span class="text-xl font-bold bg-gradient-to-r from-blue-400 to-emerald-400 bg-clip-text text-transparent">
                        TierSum
                    </span>
                </router-link>
                <nav class="flex items-center gap-1 flex-wrap justify-end">
                    <router-link to="/" custom v-slot="{ navigate, isActive }">
                        <button @click="navigate" :class="['btn btn-ghost btn-sm', isActive ? 'text-blue-400 bg-blue-500/10' : 'text-slate-400']">
                            Search
                        </button>
                    </router-link>
                    <router-link to="/docs" custom v-slot="{ navigate, isActive }">
                        <button @click="navigate" :class="['btn btn-ghost btn-sm', isActive ? 'text-blue-400 bg-blue-500/10' : 'text-slate-400']">
                            Documents
                        </button>
                    </router-link>
                    <router-link to="/tags" custom v-slot="{ navigate, isActive }">
                        <button @click="navigate" :class="['btn btn-ghost btn-sm', isActive ? 'text-blue-400 bg-blue-500/10' : 'text-slate-400']">
                            Tags
                        </button>
                    </router-link>
                    <router-link to="/observability" custom v-slot="{ navigate, isActive }">
                        <button @click="navigate" :class="['btn btn-ghost btn-sm', isActive ? 'text-blue-400 bg-blue-500/10' : 'text-slate-400']">
                            Observability
                        </button>
                    </router-link>
                    <template v-if="me">
                        <div class="dropdown dropdown-end">
                            <div tabindex="0" role="button" :class="['btn btn-ghost btn-sm gap-1', securityMenuActive ? 'text-emerald-300 bg-emerald-500/10' : 'text-slate-300']">
                                <span>Security</span>
                                <svg class="w-3 h-3 opacity-60" fill="currentColor" viewBox="0 0 20 20"><path d="M5.23 7.21a.75.75 0 011.06.02L10 11.17l3.71-3.94a.75.75 0 111.08 1.04l-4.24 4.5a.75.75 0 01-1.08 0L5.25 8.27a.75.75 0 01-.02-1.06z"/></svg>
                            </div>
                            <ul tabindex="0" class="dropdown-content menu z-[100] mt-2 w-64 rounded-box border border-slate-700 bg-slate-900 p-2 shadow-xl">
                                <li>
                                    <router-link to="/settings" custom v-slot="{ href, navigate, isActive }">
                                        <a
                                            :href="href"
                                            class="flex flex-col items-start gap-0 py-2 px-2 !h-auto rounded-lg hover:bg-slate-800 cursor-pointer transition-colors"
                                            :class="isActive ? 'bg-emerald-500/15 ring-1 ring-emerald-600/40' : ''"
                                            @click="(e) => navigate(e)"
                                        >
                                            <span class="font-medium" :class="isActive ? 'text-emerald-100' : 'text-slate-100'">Devices & sessions</span>
                                            <span class="text-xs text-slate-500">Bound browsers, rename or sign out devices</span>
                                        </a>
                                    </router-link>
                                </li>
                                <li v-if="isBrowserAdminRole(me.role)">
                                    <router-link to="/admin" custom v-slot="{ href, navigate, isActive }">
                                        <a
                                            :href="href"
                                            class="flex flex-col items-start gap-0 py-2 px-2 !h-auto rounded-lg hover:bg-slate-800 cursor-pointer transition-colors"
                                            :class="isActive ? 'bg-amber-500/15 ring-1 ring-amber-600/40' : ''"
                                            @click="(e) => navigate(e)"
                                        >
                                            <span class="font-medium" :class="isActive ? 'text-amber-100' : 'text-amber-200'">Users & API keys</span>
                                            <span class="text-xs text-slate-500">Human access tokens and service keys</span>
                                        </a>
                                    </router-link>
                                </li>
                            </ul>
                        </div>
                        <button type="button" class="btn btn-ghost btn-sm text-slate-400" @click="logout">Logout</button>
                    </template>
                </nav>
            </div>
        </header>
    `
};
