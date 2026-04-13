import { createApp } from 'vue';
import { createRouter, createWebHistory } from 'vue-router';

import { AppHeader } from './components/AppHeader.js';
import { SearchPage } from './pages/SearchPage.js';
import { DocumentCreatePage } from './pages/DocumentCreatePage.js';
import { DocumentsPage } from './pages/DocumentsPage.js';
import { DocumentDetailPage } from './pages/DocumentDetailPage.js';
import { TagsPage } from './pages/TagsPage.js';
import { ObservabilityPage } from './pages/ObservabilityPage.js';
import { ProductIntroPage } from './pages/ProductIntroPage.js';
import { InitPage } from './pages/InitPage.js';
import { LoginPage } from './pages/LoginPage.js';
import { SettingsPage } from './pages/SettingsPage.js';
import { AdminPage } from './pages/AdminPage.js';
import { AdminConfigPage } from './pages/AdminConfigPage.js';
import { apiClient, isBrowserAdminRole } from './api_client.js';

const routes = [
    { path: '/init', component: InitPage },
    { path: '/login', component: LoginPage },
    { path: '/settings', component: SettingsPage },
    { path: '/admin/config', component: AdminConfigPage },
    { path: '/admin', component: AdminPage },
    { path: '/', component: SearchPage },
    { path: '/docs', component: DocumentsPage },
    { path: '/docs/new', component: DocumentCreatePage },
    { path: '/docs/:id', component: DocumentDetailPage, props: true },
    { path: '/tags', component: TagsPage },
    { path: '/about', component: ProductIntroPage },
    { path: '/observability', component: ObservabilityPage },
    { path: '/monitoring', redirect: '/observability' }
];

const router = createRouter({
    history: createWebHistory(),
    routes
});

router.beforeEach(async (to, _from, next) => {
    let st;
    try {
        st = await apiClient.getSystemStatus();
    } catch {
        next();
        return;
    }
    if (!st.initialized) {
        if (to.path !== '/init') {
            next('/init');
            return;
        }
        next();
        return;
    }
    if (to.path === '/init') {
        next('/');
        return;
    }
    if (to.path === '/login') {
        next();
        return;
    }
    // Public product overview (no session) once the system is initialized.
    if (to.path === '/about') {
        next();
        return;
    }
    try {
        const me = await apiClient.getProfile();
        if (to.path.startsWith('/admin') && !isBrowserAdminRole(me.role)) {
            next('/');
            return;
        }
    } catch {
        next('/login');
        return;
    }
    next();
});

const App = {
    components: { AppHeader },
    template: `
        <div class="min-h-screen bg-slate-950">
            <AppHeader />
            <router-view />
        </div>
    `
};

const app = createApp(App).use(router);
app.config.errorHandler = (err, instance, info) => {
    console.error(err, info);
    const root = document.getElementById('app');
    if (root && !root.querySelector('[data-vue-error]')) {
        const pre = document.createElement('pre');
        pre.dataset.vueError = '1';
        pre.style.cssText =
            'color:#f87171;padding:1rem;font-family:monospace;white-space:pre-wrap;border-top:1px solid #334155';
        pre.textContent = 'Vue error: ' + (err && err.message ? err.message : String(err)) + '\n' + String(info);
        root.appendChild(pre);
    }
};
router.isReady().then(() => {
    app.mount('#app');
}).catch((err) => {
    const root = document.getElementById('app');
    if (root) {
        root.innerHTML =
            '<pre style="color:#f87171;padding:1rem;font-family:monospace;white-space:pre-wrap">' +
            'Router failed to start: ' +
            String(err && err.message ? err.message : err) +
            '</pre>';
    }
});
