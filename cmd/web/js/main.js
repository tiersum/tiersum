import { createApp } from 'vue';
import { createRouter, createWebHistory } from 'vue-router';

import { AppHeader } from './components/AppHeader.js';
import { SearchPage } from './pages/SearchPage.js';
import { DocumentCreatePage } from './pages/DocumentCreatePage.js';
import { DocumentsPage } from './pages/DocumentsPage.js';
import { DocumentDetailPage } from './pages/DocumentDetailPage.js';
import { TagsPage } from './pages/TagsPage.js';
import { ObservabilityPage } from './pages/ObservabilityPage.js';

const routes = [
    { path: '/', component: SearchPage },
    { path: '/docs', component: DocumentsPage },
    { path: '/docs/new', component: DocumentCreatePage },
    { path: '/docs/:id', component: DocumentDetailPage, props: true },
    { path: '/tags', component: TagsPage },
    { path: '/observability', component: ObservabilityPage },
    { path: '/monitoring', redirect: '/observability' }
];

const router = createRouter({
    // HTML5 history: /tags, /docs work with server fallback to index.html (see cmd/main.go NoRoute).
    history: createWebHistory(),
    routes
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
        pre.style.cssText = 'color:#f87171;padding:1rem;font-family:monospace;white-space:pre-wrap;border-top:1px solid #334155';
        pre.textContent = 'Vue error: ' + (err && err.message ? err.message : String(err)) + '\n' + String(info);
        root.appendChild(pre);
    }
};
router.isReady().then(() => {
    app.mount('#app');
}).catch((err) => {
    const root = document.getElementById('app');
    if (root) {
        root.innerHTML = '<pre style="color:#f87171;padding:1rem;font-family:monospace;white-space:pre-wrap">' +
            'Router failed to start: ' + String(err && err.message ? err.message : err) + '</pre>';
    }
});
