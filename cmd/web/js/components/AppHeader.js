import { apiClient, isBrowserAdminRole } from '../api_client.js';
import { setLocale, getLocale, getAvailableLocales } from '../i18n.js';

export const AppHeader = {
    data() {
        return { me: null, locale: getLocale() };
    },
    computed: {
        managementMenuActive() {
            const p = this.$route?.path || '';
            return (
                p === '/settings' ||
                p.startsWith('/admin') ||
                p === '/observability'
            );
        },
        /** Highlight Library for browse, create, and detail routes. */
        libraryNavActive() {
            const p = this.$route?.path || '';
            return p === '/library' || p === '/docs' || (p.startsWith('/docs/') && !p.match(/^\/docs\/(index|features|documentation|about)$/));
        },
        /** Two-letter (or one) initials from username for the header avatar. */
        userAvatarInitials() {
            const u = (this.me?.username || '').trim();
            if (!u) return '?';
            const parts = u.split(/[^a-zA-Z0-9\u00C0-\u024F]+/).filter(Boolean);
            if (parts.length >= 2) {
                const a = parts[0].charAt(0);
                const b = parts[parts.length - 1].charAt(0);
                return (a + b).toUpperCase();
            }
            const up = u.toUpperCase();
            return up.length <= 2 ? up : up.slice(0, 2);
        },
        /** Stable hue from username for a readable avatar background. */
        userAvatarStyle() {
            const u = (this.me?.username || 'user').trim() || 'user';
            let h = 0;
            for (let i = 0; i < u.length; i++) {
                h = (h * 31 + u.charCodeAt(i)) >>> 0;
            }
            const hue = h % 360;
            return { backgroundColor: `hsl(${hue} 52% 44%)`, color: '#0f172a' };
        },
        availableLocales() {
            return getAvailableLocales();
        }
    },
    async mounted() {
        await this.refreshMe();
    },
    watch: {
        // Login updates the session cookie, but this header component can stay mounted across routes.
        // Refresh the profile on navigation so admin menus and logout appear immediately after sign-in.
        async '$route.path'() {
            await this.refreshMe({ soft: true });
        }
    },
    methods: {
        isBrowserAdminRole,
        async refreshMe(opts = {}) {
            const soft = opts && opts.soft === true
            try {
                const st = await apiClient.getSystemStatus();
                if (!st.initialized) {
                    this.me = null;
                    return;
                }
                this.me = await apiClient.getProfile();
            } catch (e) {
                if (!soft) this.me = null;
            }
        },
        async logout() {
            try {
                await apiClient.logout();
            } catch {
                /* ignore */
            }
            this.me = null;
            window.location.assign('/ui/login');
        },
        switchLocale(locale) {
            setLocale(locale);
            this.locale = locale;
            window.location.reload();
        }
    },
    template: `
        <header class="border-b border-slate-800 bg-slate-950/80 backdrop-blur-md sticky top-0 z-50">
            <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center justify-between">
                <a href="https://tiersum.tech/" class="flex items-center gap-2 hover:opacity-80 transition-opacity">
                    <svg class="w-7 h-7 text-blue-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z"/>
                    </svg>
                    <span class="text-xl font-bold bg-gradient-to-r from-blue-400 to-emerald-400 bg-clip-text text-transparent">
                        TierSum
                    </span>
                </a>
                <nav class="flex items-center gap-1 flex-wrap justify-end">
                    <!-- Locale switcher -->
                    <div class="dropdown dropdown-end">
                        <div tabindex="0" role="button" class="btn btn-ghost btn-sm text-slate-400 hover:text-slate-200">
                            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 5h12M9 3v2m0 14v2m0-8h6m3-2l-3 3 3 3m-3-3H3"/>
                            </svg>
                            {{ locale === 'zh' ? '中文' : 'EN' }}
                        </div>
                        <ul tabindex="0" class="dropdown-content menu z-[100] mt-2 rounded-box border border-slate-700 bg-slate-900 p-2 shadow-xl">
                            <li v-for="loc in availableLocales" :key="loc">
                                <a
                                    :class="['cursor-pointer', locale === loc ? 'text-blue-300' : 'text-slate-300']"
                                    @click.prevent="switchLocale(loc)"
                                >
                                    {{ loc === 'zh' ? '中文' : 'English' }}
                                </a>
                            </li>
                        </ul>
                    </div>
                    <template v-if="me">
                        <div class="w-px h-6 bg-slate-700 mx-1"></div>
                        <router-link to="/search" custom v-slot="{ navigate, isActive }">
                            <button @click="navigate" :class="['btn btn-ghost btn-sm', isActive ? 'text-blue-400 bg-blue-500/10' : 'text-slate-400']">
                                {{ $t('navSearch') }}
                            </button>
                        </router-link>
                        <router-link to="/library" custom v-slot="{ navigate }">
                            <button @click="navigate" :class="['btn btn-ghost btn-sm', libraryNavActive ? 'text-blue-400 bg-blue-500/10' : 'text-slate-400']">
                                {{ $t('navLibrary') }}
                            </button>
                        </router-link>
                        <div class="dropdown dropdown-end">
                            <div tabindex="0" role="button" :class="['btn btn-ghost btn-sm gap-1', managementMenuActive ? 'text-sky-300 bg-sky-500/10' : 'text-slate-300']">
                                <span>{{ $t('navManagement') }}</span>
                                <svg class="w-3 h-3 opacity-60" fill="currentColor" viewBox="0 0 20 20"><path d="M5.23 7.21a.75.75 0 011.06.02L10 11.17l3.71-3.94a.75.75 0 111.08 1.04l-4.24 4.5a.75.75 0 01-1.08 0L5.25 8.27a.75.75 0 01-.02-1.06z"/></svg>
                            </div>
                            <ul tabindex="0" class="dropdown-content menu z-[100] mt-2 w-72 rounded-box border border-slate-700 bg-slate-900 p-2 shadow-xl">
                                <li v-if="isBrowserAdminRole(me?.role)">
                                    <router-link to="/observability" custom v-slot="{ href, navigate, isActive }">
                                        <a
                                            :href="href"
                                            class="flex flex-col items-start gap-0 py-2 px-2 !h-auto rounded-lg hover:bg-slate-800 cursor-pointer transition-colors"
                                            :class="isActive ? 'bg-sky-500/15 ring-1 ring-sky-600/40' : ''"
                                            @click="(e) => navigate(e)"
                                        >
                                            <span class="font-medium" :class="isActive ? 'text-sky-100' : 'text-slate-100'">{{ $t('navObservability') }}</span>
                                            <span class="text-xs text-slate-500">{{ $t('observabilityDesc') }}</span>
                                        </a>
                                    </router-link>
                                </li>
                                <li>
                                    <router-link to="/settings" custom v-slot="{ href, navigate, isActive }">
                                        <a
                                            :href="href"
                                            class="flex flex-col items-start gap-0 py-2 px-2 !h-auto rounded-lg hover:bg-slate-800 cursor-pointer transition-colors"
                                            :class="isActive ? 'bg-emerald-500/15 ring-1 ring-emerald-600/40' : ''"
                                            @click="(e) => navigate(e)"
                                        >
                                            <span class="font-medium" :class="isActive ? 'text-emerald-100' : 'text-slate-100'">{{ $t('navDevicesSessions') }}</span>
                                            <span class="text-xs text-slate-500">{{ $t('devicesSessionsDesc') }}</span>
                                        </a>
                                    </router-link>
                                </li>
                                <li v-if="isBrowserAdminRole(me.role)">
                                    <router-link to="/admin" custom v-slot="{ href, navigate, isActive }">
                                        <a
                                            :href="href"
                                            class="flex flex-col items-start gap-0 py-2 px-2 !h-auto rounded-lg hover:bg-slate-800 cursor-pointer transition-colors"
                                            :class="isActive && $route.path === '/admin' ? 'bg-amber-500/15 ring-1 ring-amber-600/40' : ''"
                                            @click="(e) => navigate(e)"
                                        >
                                            <span class="font-medium" :class="isActive && $route.path === '/admin' ? 'text-amber-100' : 'text-amber-200'">{{ $t('navUsersAPIKeys') }}</span>
                                            <span class="text-xs text-slate-500">{{ $t('usersAPIKeysDesc') }}</span>
                                        </a>
                                    </router-link>
                                </li>
                                <li v-if="isBrowserAdminRole(me.role)">
                                    <router-link to="/admin/config" custom v-slot="{ href, navigate, isActive }">
                                        <a
                                            :href="href"
                                            class="flex flex-col items-start gap-0 py-2 px-2 !h-auto rounded-lg hover:bg-slate-800 cursor-pointer transition-colors"
                                            :class="isActive ? 'bg-violet-500/15 ring-1 ring-violet-600/40' : ''"
                                            @click="(e) => navigate(e)"
                                        >
                                            <span class="font-medium" :class="isActive ? 'text-violet-100' : 'text-slate-100'">{{ $t('navConfiguration') }}</span>
                                            <span class="text-xs text-slate-500">{{ $t('configurationDesc') }}</span>
                                        </a>
                                    </router-link>
                                </li>
                            </ul>
                        </div>
                        <div class="flex items-center gap-2 shrink-0 ml-1">
                            <span
                                class="flex h-8 w-8 shrink-0 select-none items-center justify-center rounded-full text-[11px] font-bold leading-none ring-1 ring-white/15 shadow-sm"
                                :style="userAvatarStyle"
                                role="img"
                                :aria-label="$t('signedInAs') + ' ' + (me.username || 'user')"
                                :title="(me.username || '') + (me.role ? ' (' + me.role + ')' : '')"
                            >{{ userAvatarInitials }}</span>
                            <button type="button" class="btn btn-ghost btn-sm text-slate-400" @click="logout">{{ $t('navLogout') }}</button>
                        </div>
                    </template>
                </nav>
            </div>
        </header>
    `
};
