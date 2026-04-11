import { marked } from './vendor/marked.esm.js';

export function parseMarkdown(content) {
    if (!content) return '';
    try {
        return marked.parse(content);
    } catch {
        return '';
    }
}

export function parseMarkdownPreview(rawContent, emptyHtml) {
    const t = (rawContent || '').trim();
    if (!t) {
        return emptyHtml || '<p class="text-slate-500 italic">Preview appears here as you type Markdown.</p>';
    }
    try {
        return marked.parse(t);
    } catch {
        return '<p class="text-red-400">Invalid Markdown.</p>';
    }
}

export function parseMarkdownOrError(text, errorHtml) {
    if (!text) return '';
    try {
        return marked.parse(text);
    } catch {
        return errorHtml || '<p class="text-red-400">Failed to render markdown.</p>';
    }
}
