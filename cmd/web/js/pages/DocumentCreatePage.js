import { apiClient } from '../api_client.js';
import { parseMarkdownPreview } from '../markdown.js';

export const DocumentCreatePage = {
    data() {
        return {
            newDoc: {
                title: '',
                content: '',
                format: 'markdown',
                tags: [],
                ingest_mode: 'auto'
            },
            tagInput: '',
            submitting: false,
            contentDragActive: false,
            errorMessage: ''
        };
    },
    computed: {
        canSubmit() {
            return Boolean(this.newDoc.title.trim() && this.newDoc.content.trim()) && !this.submitting;
        },
        contentChars() {
            return [...this.newDoc.content].length;
        }
    },
    methods: {
        clearError() {
            this.errorMessage = '';
        },
        addTag() {
            if (this.tagInput.trim() && !this.newDoc.tags.includes(this.tagInput.trim())) {
                this.newDoc.tags.push(this.tagInput.trim());
                this.tagInput = '';
            }
        },
        removeTag(index) {
            this.newDoc.tags.splice(index, 1);
        },
        renderPreview(text) {
            return parseMarkdownPreview(text);
        },
        async submitDocument() {
            if (!this.canSubmit) return;
            this.clearError();
            this.submitting = true;
            try {
                const payload = {
                    title: this.newDoc.title.trim(),
                    content: this.newDoc.content,
                    format: (this.newDoc.format || 'markdown').toLowerCase() === 'md' ? 'md' : 'markdown'
                };
                if (this.newDoc.tags.length) {
                    payload.tags = [...this.newDoc.tags];
                }
                const mode = (this.newDoc.ingest_mode || 'auto').toLowerCase();
                if (mode === 'hot' || mode === 'cold') {
                    payload.ingest_mode = mode;
                }
                const created = await apiClient.createDocument(payload);
                const id = created?.id || created?.ID;
                if (id) {
                    this.$router.push(`/docs/${id}`);
                } else {
                    this.$router.push('/library');
                }
            } catch (error) {
                console.error('Failed to create document:', error);
                this.errorMessage =
                    (error && error.message) || this.$t('createSubmitError');
            } finally {
                this.submitting = false;
            }
        },
        goBack() {
            this.$router.push('/library');
        },
        triggerMarkdownFilePick() {
            this.$refs.markdownFile?.click();
        },
        transferHasFiles(event) {
            return Boolean(event.dataTransfer?.types?.includes('Files'));
        },
        isMarkdownLike(file) {
            if (!file || !file.name) return false;
            const t = (file.type || '').toLowerCase();
            if (t === 'text/markdown' || t === 'text/x-markdown') return true;
            return /\.(md|markdown|mdown|mkd|mkdn|mdtxt)$/i.test(file.name);
        },
        pickMarkdownFromFileList(fileList) {
            if (!fileList || !fileList.length) return null;
            return Array.from(fileList).find((f) => this.isMarkdownLike(f)) || null;
        },
        async applyMarkdownFromFile(file) {
            const text = await file.text();
            this.newDoc.content = text;
            if (!this.newDoc.title.trim()) {
                const base = file.name.replace(/\.(md|markdown|mdown|mkd|mkdn|mdtxt)$/i, '').trim();
                if (base) {
                    this.newDoc.title = base;
                }
            }
        },
        async onMarkdownFileSelected(event) {
            const input = event.target;
            const file = input?.files?.[0];
            if (!file) return;
            try {
                await this.applyMarkdownFromFile(file);
            } catch (err) {
                console.error('Failed to read file:', err);
                this.errorMessage = this.$t('createReadError', { error: err.message || String(err) });
            } finally {
                input.value = '';
            }
        },
        onContentDragOver(event) {
            if (!this.transferHasFiles(event)) return;
            event.preventDefault();
            event.dataTransfer.dropEffect = 'copy';
            this.contentDragActive = true;
        },
        onContentDragLeave(event) {
            const zone = this.$refs.markdownDropZone;
            const rt = event.relatedTarget;
            if (zone && rt instanceof Node && zone.contains(rt)) return;
            this.contentDragActive = false;
        },
        async onContentDrop(event) {
            this.contentDragActive = false;
            event.preventDefault();
            if (!this.transferHasFiles(event)) return;
            const file = this.pickMarkdownFromFileList(event.dataTransfer.files);
            if (!file) {
                this.errorMessage = this.$t('createDropMarkdown');
                return;
            }
            try {
                await this.applyMarkdownFromFile(file);
                this.clearError();
            } catch (err) {
                console.error('Failed to read dropped file:', err);
                this.errorMessage = this.$t('createReadError', { error: err.message || String(err) });
            }
        }
    },
    template: `
        <div class="min-h-screen bg-slate-950">
            <main class="max-w-[1800px] mx-auto px-4 sm:px-6 lg:px-8 py-6 pb-16">
                <div v-if="errorMessage" role="alert" class="alert alert-error border border-red-900/60 bg-red-950/50 text-red-100 mb-4 flex flex-row items-start justify-between gap-3">
                    <span class="text-sm leading-snug pt-0.5">{{ errorMessage }}</span>
                    <button type="button" class="btn btn-ghost btn-xs shrink-0 text-red-200" @click="clearError">{{ $t('dismiss') }}</button>
                </div>
                <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 mb-6">
                    <div class="flex items-center gap-3">
                        <button type="button" @click="goBack" class="btn btn-ghost btn-sm gap-2 text-slate-400">
                            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7"/>
                            </svg>
                            {{ $t('createBackToList') }}
                        </button>
                        <h1 class="text-2xl sm:text-3xl font-bold text-slate-100">{{ $t('createTitle') }}</h1>
                    </div>
                    <div class="flex flex-wrap items-center gap-2">
                        <label class="label cursor-pointer gap-2 py-0 flex-nowrap">
                            <span class="label-text text-slate-400 text-sm whitespace-nowrap">{{ $t('createIngest') }}</span>
                            <select v-model="newDoc.ingest_mode" class="select select-bordered select-sm bg-slate-800/80 border-slate-700 text-slate-100 min-w-[11rem]">
                                <option value="auto">{{ $t('createAuto') }}</option>
                                <option value="hot">{{ $t('createHot') }}</option>
                                <option value="cold">{{ $t('createCold') }}</option>
                            </select>
                        </label>
                        <button type="button"
                            @click="submitDocument"
                            :disabled="!canSubmit"
                            class="btn btn-primary">
                            <span v-if="submitting" class="loading loading-spinner loading-sm mr-2"></span>
                            {{ $t('createSubmit') }}
                        </button>
                    </div>
                </div>

                <div class="grid grid-cols-1 xl:grid-cols-2 gap-6 min-h-[calc(100vh-12rem)]">
                    <div class="flex flex-col gap-4 min-h-0">
                        <div class="card bg-slate-900/50 border border-slate-800 flex-1 flex flex-col min-h-[520px] xl:min-h-[calc(100vh-14rem)]">
                            <div class="card-body flex flex-col flex-1 min-h-0 gap-4">
                                <div>
                                    <label class="label"><span class="label-text text-slate-300">{{ $t('createTitleLabel') }}</span></label>
                                    <input v-model="newDoc.title" type="text" :placeholder="$t('createTitlePlaceholder')"
                                        class="input input-bordered w-full bg-slate-800/80 border-slate-700 text-slate-100"
                                        @input="clearError" />
                                </div>
                                <div>
                                    <label class="label"><span class="label-text text-slate-300">{{ $t('createTags') }}</span></label>
                                    <div class="flex gap-2 mb-2">
                                        <input v-model="tagInput" @keydown.enter.prevent="addTag" type="text"
                                            :placeholder="$t('createTagPlaceholder')"
                                            class="input input-bordered flex-1 bg-slate-800/80 border-slate-700 text-slate-100" />
                                        <button type="button" @click="addTag" class="btn btn-outline border-slate-600">{{ $t('createAddTag') }}</button>
                                    </div>
                                    <div class="flex flex-wrap gap-2">
                                        <span v-for="(tag, index) in newDoc.tags" :key="index" class="badge badge-primary gap-1">
                                            {{ tag }}
                                            <button type="button" @click="removeTag(index)" class="hover:text-white" aria-label="Remove tag">×</button>
                                        </span>
                                    </div>
                                </div>
                                <div
                                    ref="markdownDropZone"
                                    class="flex flex-col flex-1 min-h-0 rounded-xl transition-colors"
                                    :class="contentDragActive ? 'ring-2 ring-blue-500/50 ring-offset-2 ring-offset-slate-950 bg-slate-800/25' : ''"
                                    @dragover.capture.prevent="onContentDragOver"
                                    @dragleave="onContentDragLeave"
                                    @drop.capture.prevent="onContentDrop"
                                >
                                    <div class="label py-0 flex flex-wrap items-start justify-between gap-x-4 gap-y-2 shrink-0">
                                        <div class="min-w-0 flex-1">
                                            <span class="label-text text-slate-300">{{ $t('createContentLabel') }}</span>
                                            <p class="text-xs text-slate-500 mt-1 mb-0 max-w-xl leading-snug">
                                                <strong class="text-slate-400 font-medium">{{ $t('createChooseFile') }}</strong> {{ $t('createContentHint') }}
                                            </p>
                                        </div>
                                        <div class="flex items-center gap-2 shrink-0">
                                            <input
                                                ref="markdownFile"
                                                type="file"
                                                accept=".md,.markdown,.mdown,.mkd,.mkdn,.mdtxt,text/markdown,text/x-markdown"
                                                class="hidden"
                                                @change="onMarkdownFileSelected"
                                            />
                                            <button type="button" @click="triggerMarkdownFilePick" class="btn btn-sm btn-outline border-slate-600 gap-1.5">
                                                <svg class="w-4 h-4 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
                                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12"/>
                                                </svg>
                                                {{ $t('createChooseFile') }}
                                            </button>
                                        </div>
                                    </div>
                                    <div class="flex items-center justify-between gap-2 text-xs text-slate-500 shrink-0 -mt-1 mb-1">
                                        <span>{{ $t('createChars', { count: contentChars }) }}</span>
                                    </div>
                                    <textarea v-model="newDoc.content"
                                        :placeholder="$t('createContentPlaceholder')"
                                        class="textarea textarea-bordered flex-1 min-h-[280px] w-full bg-slate-800/80 border-slate-700 text-slate-100 font-mono text-sm leading-relaxed resize-y"
                                        @input="clearError"></textarea>
                                </div>
                            </div>
                        </div>
                    </div>

                    <div class="flex flex-col min-h-0 xl:sticky xl:top-20 xl:self-start xl:max-h-[calc(100vh-6rem)]">
                        <div class="card bg-slate-900/50 border border-slate-800 h-full min-h-[320px] xl:max-h-[calc(100vh-14rem)] flex flex-col">
                            <div class="px-4 py-3 border-b border-slate-800 flex items-center justify-between shrink-0">
                                <h2 class="text-sm font-semibold text-slate-400 uppercase tracking-wide">{{ $t('createPreview') }}</h2>
                                <span class="text-xs text-slate-600">{{ $t('createLive') }}</span>
                            </div>
                            <div class="card-body overflow-y-auto flex-1 min-h-0 pt-4">
                                <article class="markdown-body max-w-none px-0 py-0 text-sm sm:text-[15px]">
                                    <div v-html="renderPreview(newDoc.content)"></div>
                                </article>
                            </div>
                        </div>
                    </div>
                </div>
            </main>
        </div>
    `
};
