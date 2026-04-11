import { createApp } from 'vue';
import { createRouter, createWebHashHistory } from 'vue-router';

import { AppHeader } from './components/AppHeader.js';
import { SearchPage } from './pages/SearchPage.js';
import { DocumentCreatePage } from './pages/DocumentCreatePage.js';
import { DocumentsPage } from './pages/DocumentsPage.js';
import { DocumentDetailPage } from './pages/DocumentDetailPage.js';
import { TagsPage } from './pages/TagsPage.js';

const routes = [
    { path: '/', component: SearchPage },
    { path: '/docs', component: DocumentsPage },
    { path: '/docs/new', component: DocumentCreatePage },
    { path: '/docs/:id', component: DocumentDetailPage, props: true },
    { path: '/tags', component: TagsPage }
];

const router = createRouter({
    history: createWebHashHistory(),
    routes
});

const App = {
    components: { AppHeader },
    template: `
        <div class="min-h-screen bg-slate-950">
            <AppHeader />
            <router-view v-slot="{ Component }">
                <transition name="fade" mode="out-in">
                    <component :is="Component" />
                </transition>
            </router-view>
        </div>
    `
};

createApp(App).use(router).mount('#app');
