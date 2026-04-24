import { apiClient, isBrowserViewerRole } from '../api_client.js';
import { Icon } from '../components/Icon.js';
import { LibraryNav } from '../components/library/LibraryNav.js';
import { DocumentList } from '../components/library/DocumentList.js';
import {
    BROWSE_ALL,
    BROWSE_TOPIC,
    filterDocuments,
    buildCatalogTagNameSet
} from '../utils/libraryFilters.js';

export const LibraryPage = {
    components: { Icon, LibraryNav, DocumentList },
    data() {
        return {
            profile: null,
            documents: [],
            topics: [],
            tags: [],
            browseMode: BROWSE_ALL,
            selectedTopic: null,
            selectedCatalogTagName: null,
            searchQuery: '',
            loading: true,
            tagsPanelLoading: false,
            loadError: null,
            regrouping: false
        };
    },
    computed: {
        isViewer() {
            return isBrowserViewerRole(this.profile?.role);
        },
        catalogTagNameSet() {
            return buildCatalogTagNameSet(this.tags);
        },
        filteredDocs() {
            return filterDocuments(this.documents, {
                browseMode: this.browseMode,
                searchQuery: this.searchQuery,
                selectedTopic: this.selectedTopic,
                selectedCatalogTagName: this.selectedCatalogTagName,
                catalogTagNameSet: this.catalogTagNameSet
            });
        }
    },
    async mounted() {
        try {
            this.profile = await apiClient.getProfile();
        } catch {
            this.profile = null;
        }
        await this.loadData();
    },
    methods: {
        pickDefaultTopic() {
            const list = Array.isArray(this.topics) ? this.topics : [];
            return list.find((g) => g && g.id != null && String(g.id).trim() !== '') || null;
        },
        async loadTagsForSelectedTopic() {
            if (!this.selectedTopic || this.selectedTopic.id == null || String(this.selectedTopic.id).trim() === '') {
                this.tags = [];
                return;
            }
            this.tagsPanelLoading = true;
            try {
                this.tags = await apiClient.getTags({
                    topic_ids: [String(this.selectedTopic.id)],
                    max_results: 5000
                });
            } catch (e) {
                console.error('Failed to load tags for topic:', e);
                this.tags = [];
            } finally {
                this.tagsPanelLoading = false;
            }
        },
        async loadData() {
            this.loading = true;
            this.loadError = null;
            try {
                try {
                    this.profile = await apiClient.getProfile();
                } catch {
                    this.profile = null;
                }
                const prevTopicId =
                    this.browseMode === BROWSE_TOPIC && this.selectedTopic && this.selectedTopic.id != null
                        ? String(this.selectedTopic.id)
                        : '';
                const [docs, rawTopics] = await Promise.all([apiClient.getDocuments(), apiClient.getTopics()]);
                this.documents = Array.isArray(docs) ? docs : [];
                this.topics = Array.isArray(rawTopics) ? rawTopics : [];
                if (this.browseMode === BROWSE_TOPIC && prevTopicId) {
                    const still = this.topics.find((g) => g && String(g.id) === prevTopicId);
                    this.selectedCatalogTagName = null;
                    if (still) {
                        this.selectedTopic = still;
                        await this.loadTagsForSelectedTopic();
                    } else {
                        this.selectedTopic = this.pickDefaultTopic();
                        if (this.selectedTopic) await this.loadTagsForSelectedTopic();
                        else {
                            this.tags = [];
                            this.browseMode = BROWSE_ALL;
                        }
                    }
                }
            } catch (e) {
                console.error('Failed to reload library:', e);
                this.loadError = e && e.message ? e.message : String(e);
            } finally {
                this.loading = false;
            }
        },
        setBrowseMode(mode) {
            this.browseMode = mode;
            if (mode !== BROWSE_TOPIC) {
                this.selectedTopic = null;
                this.selectedCatalogTagName = null;
                this.tags = [];
            }
        },
        async selectTopic(topic) {
            this.browseMode = BROWSE_TOPIC;
            this.selectedTopic = topic;
            this.selectedCatalogTagName = null;
            await this.loadTagsForSelectedTopic();
        },
        selectCatalogTag(tag) {
            const name = tag && tag.name != null ? String(tag.name) : '';
            if (!name) return;
            if (this.selectedCatalogTagName === name) {
                this.selectedCatalogTagName = null;
            } else {
                this.selectedCatalogTagName = name;
            }
        },
        clearCatalogTagFilter() {
            this.selectedCatalogTagName = null;
        },
        async triggerRegroup() {
            this.regrouping = true;
            try {
                await apiClient.triggerTopicRegroup();
                await this.loadData();
            } catch (error) {
                console.error('Failed to regroup:', error);
                this.loadError = this.$t('libraryRegroup') + ' failed: ' + (error && error.message ? error.message : String(error));
            } finally {
                this.regrouping = false;
            }
        },
        goToDoc(id) {
            this.$router.push(`/docs/${id}`);
        }
    },
    template: `
        <div class="min-h-screen bg-slate-950">
            <main class="max-w-[1600px] mx-auto px-4 sm:px-6 lg:px-8 py-8">
                <!-- Header -->
                <div class="flex flex-col sm:flex-row sm:items-start sm:justify-between gap-4 mb-6">
                    <div>
                        <h1 class="text-3xl font-bold text-slate-100 mb-2">{{ $t('libraryTitle') }}</h1>
                        <p class="text-slate-400">
                            {{ $t('librarySubtitle') }}
                        </p>
                    </div>
                    <div class="flex flex-wrap items-center gap-2 shrink-0">
                        <router-link v-if="!isViewer" to="/docs/new" class="btn btn-primary btn-sm">
                            <icon name="plus" class-name="w-5 h-5 mr-1" />
                            {{ $t('libraryAddDoc') }}
                        </router-link>
                    </div>
                </div>

                <!-- Error banner -->
                <div v-if="loadError && !loading" class="alert alert-error alert-soft mb-6">
                    <span>{{ loadError }}</span>
                    <button type="button" class="btn btn-sm btn-ghost" @click="loadData">{{ $t('retry') }}</button>
                </div>

                <!-- Main content: fixed two-column layout -->
                <div class="grid grid-cols-1 lg:grid-cols-12 gap-6 lg:h-[calc(100vh-260px)]">
                    <!-- Left: navigation -->
                    <div class="col-span-12 lg:col-span-3 h-full">
                        <library-nav
                            :loading="loading"
                            :browse-mode="browseMode"
                            :topics="topics"
                            :selected-topic="selectedTopic"
                            :is-viewer="isViewer"
                            :regrouping="regrouping"
                            @set-browse-mode="setBrowseMode"
                            @select-topic="selectTopic"
                            @regroup="triggerRegroup"
                        />
                    </div>

                    <!-- Right: documents -->
                    <div class="col-span-12 lg:col-span-9 h-full">
                        <document-list
                            :loading="loading"
                            :docs="filteredDocs"
                            :browse-mode="browseMode"
                            :selected-topic="selectedTopic"
                            :selected-catalog-tag-name="selectedCatalogTagName"
                            :is-viewer="isViewer"
                            :tags="tags"
                            :tags-loading="tagsPanelLoading"
                            @update:search-query="searchQuery = $event"
                            @open-doc="goToDoc"
                            @clear-catalog-tag="clearCatalogTagFilter"
                            @select-catalog-tag="selectCatalogTag"
                        />
                    </div>
                </div>
            </main>
        </div>
    `
};
