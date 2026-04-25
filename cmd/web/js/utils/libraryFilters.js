const BROWSE_ALL = 'all';
const BROWSE_COLD = 'cold';
const BROWSE_UNTAGGED = 'untagged';
const BROWSE_TOPIC = 'topic';

export function filterDocuments(docs, { browseMode, searchQuery, selectedTopic, selectedCatalogTagName }) {
    let list = Array.isArray(docs) ? [...docs] : [];
    const q = (searchQuery || '').trim().toLowerCase();
    if (q) {
        list = list.filter(
            (doc) =>
                doc.title?.toLowerCase().includes(q) ||
                doc.tags?.some((tag) => String(tag).toLowerCase().includes(q))
        );
    }
    if (browseMode === BROWSE_COLD) {
        list = list.filter((d) => d.status === 'cold');
    } else if (browseMode === BROWSE_UNTAGGED) {
        list = list.filter((d) => !d.tags || d.tags.length === 0);
    } else if (browseMode === BROWSE_TOPIC && selectedTopic) {
        const tagNames = selectedTopic.tag_names || [];
        const set = new Set(tagNames.map(String));
        if (selectedCatalogTagName) {
            list = list.filter((d) => d.tags?.includes(selectedCatalogTagName));
        } else if (set.size === 0) {
            list = [];
        } else {
            list = list.filter((d) => d.tags?.some((dt) => set.has(String(dt))));
        }
    }
    return list;
}

export function buildCatalogTagNameSet(tags) {
    const set = new Set();
    for (const t of tags || []) {
        const n = t && t.name != null ? String(t.name) : '';
        if (n.trim() !== '') set.add(n);
    }
    return set;
}

export { BROWSE_ALL, BROWSE_COLD, BROWSE_UNTAGGED, BROWSE_TOPIC };
