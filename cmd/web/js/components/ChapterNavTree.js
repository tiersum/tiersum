/**
 * Build a tree from chapter paths: paths are "docId/heading/..." (slashes); title may be a flat breadcrumb.
 * Navigation uses path segments; each row label is the segment for that depth (not the full title).
 */
export function buildChapterNavTree(chapters, docId) {
    const root = { segment: '', chapter: null, children: [] };
    const norm = (p) => String(p || '').replace(/\\/g, '/').trim();
    const doc = String(docId || '').trim();

    for (const ch of chapters || []) {
        const p = norm(ch.path);
        if (!p) continue;
        const prefix = doc ? `${doc}/` : '';
        let rel = p.startsWith(prefix) ? p.slice(prefix.length) : p;
        const parts = rel.split('/').map((s) => s.trim()).filter(Boolean);
        if (parts.length === 0) continue;

        let parent = root;
        for (let i = 0; i < parts.length; i++) {
            const seg = parts[i];
            let child = parent.children.find((c) => c.segment === seg);
            if (!child) {
                child = { segment: seg, chapter: null, children: [] };
                parent.children.push(child);
            }
            if (i === parts.length - 1) {
                child.chapter = ch;
            }
            parent = child;
        }
    }

    const sortNodes = (nodes) => {
        nodes.sort((a, b) =>
            String(a.segment).localeCompare(String(b.segment), undefined, { numeric: true, sensitivity: 'base' })
        );
        for (const n of nodes) {
            if (n.children?.length) sortNodes(n.children);
        }
    };
    sortNodes(root.children);
    return root.children;
}

export const ChapterNavTree = {
    name: 'ChapterNavTree',
    props: {
        nodes: { type: Array, default: () => [] },
        selectedPath: { type: String, default: '' }
    },
    emits: ['select'],
    methods: {
        isSelected(path) {
            return path && this.selectedPath === path;
        },
        onSelect(path) {
            if (path) this.$emit('select', path);
        }
    },
    template: `
        <ul class="space-y-0.5 border-l border-slate-700/50 ml-2 pl-2 first:ml-0 first:pl-0 first:border-l-0">
            <li v-for="node in nodes" :key="node.segment + '|' + (node.chapter?.path || '')" class="min-w-0">
                <button
                    v-if="node.chapter"
                    type="button"
                    @click="onSelect(node.chapter.path)"
                    :class="['group w-full flex items-start gap-2 text-left px-2 py-1.5 rounded-md text-sm transition-colors border',
                        isSelected(node.chapter.path)
                            ? 'bg-blue-500/20 text-blue-200 border-blue-500/40'
                            : 'text-slate-300 border-transparent hover:bg-slate-800/80 hover:border-slate-700/50']">
                    <span
                        class="inline-flex shrink-0 mt-0.5"
                        :class="isSelected(node.chapter.path) ? 'text-blue-400' : 'text-slate-500 group-hover:text-slate-400'"
                        aria-hidden="true">
                        <svg v-if="node.children && node.children.length" class="w-4 h-4" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" d="M2.25 12.75V12A2.25 2.25 0 0 1 4.5 9.75h15A2.25 2.25 0 0 1 21.75 12v.75m-16.5 3.75h15m-15 0V19.5A2.25 2.25 0 0 0 4.5 22.5h15a2.25 2.25 0 0 0 2.25-2.25V16.5m-18.75-3.75v-2.625A2.25 2.25 0 0 1 4.5 6h5.379a2.25 2.25 0 0 1 1.06.44l2.122 2.12a2.25 2.25 0 0 0 1.06.44H19.5a2.25 2.25 0 0 1 2.25 2.25V13.5" />
                        </svg>
                        <svg v-else class="w-4 h-4" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 0 0-3.375-3.375h-1.5a1.125 1.125 0 0 1-1.125-1.125v-1.5a3.375 3.375 0 0 0-3.375-3.375H8.25m2.25 0H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 0 0-9-9Z" />
                        </svg>
                    </span>
                    <span class="min-w-0 flex-1 break-words leading-snug">{{ node.segment }}</span>
                </button>
                <div
                    v-else
                    class="flex items-start gap-2 px-2 pt-1 pb-0.5 text-xs font-medium text-slate-500 break-words select-none">
                    <span class="inline-flex shrink-0 mt-0.5 text-slate-600" aria-hidden="true">
                        <svg class="w-4 h-4 opacity-90" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" d="m8.25 4.5 7.5 7.5-7.5 7.5" />
                        </svg>
                    </span>
                    <span class="min-w-0 flex-1 leading-snug text-slate-500">{{ node.segment }}</span>
                </div>
                <ChapterNavTree
                    v-if="node.children && node.children.length"
                    :nodes="node.children"
                    :selected-path="selectedPath"
                    @select="onSelect($event)"
                />
            </li>
        </ul>
    `
};

ChapterNavTree.components = { ChapterNavTree };
