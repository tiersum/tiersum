export const AppHeader = {
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
                <nav class="flex items-center gap-1">
                    <router-link to="/" custom v-slot="{ navigate, isActive }">
                        <button @click="navigate" :class="['btn btn-ghost btn-sm', isActive ? 'text-blue-400 bg-blue-500/10' : 'text-slate-400']">
                            <svg class="w-4 h-4 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"/>
                            </svg>
                            Search
                        </button>
                    </router-link>
                    <router-link to="/docs" custom v-slot="{ navigate, isActive }">
                        <button @click="navigate" :class="['btn btn-ghost btn-sm', isActive ? 'text-blue-400 bg-blue-500/10' : 'text-slate-400']">
                            <svg class="w-4 h-4 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"/>
                            </svg>
                            Documents
                        </button>
                    </router-link>
                    <router-link to="/tags" custom v-slot="{ navigate, isActive }">
                        <button @click="navigate" :class="['btn btn-ghost btn-sm', isActive ? 'text-blue-400 bg-blue-500/10' : 'text-slate-400']">
                            <svg class="w-4 h-4 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 7h.01M7 3h5c.512 0 1.024.195 1.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A1.994 1.994 0 013 12V7a4 4 0 014-4z"/>
                            </svg>
                            Tags
                        </button>
                    </router-link>
                    <router-link to="/monitoring" custom v-slot="{ navigate, isActive }">
                        <button @click="navigate" :class="['btn btn-ghost btn-sm', isActive ? 'text-blue-400 bg-blue-500/10' : 'text-slate-400']">
                            <svg class="w-4 h-4 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z"/>
                            </svg>
                            Monitoring
                        </button>
                    </router-link>
                </nav>
            </div>
        </header>
    `
};
