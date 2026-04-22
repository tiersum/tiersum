import { marked } from './vendor/marked.esm.js';

// Custom renderer to fix anchor links for SPA navigation
const renderer = new marked.Renderer();
const originalHeading = renderer.heading.bind(renderer);
renderer.heading = function(text, level, raw) {
    // Generate ID from heading text (same logic as GitHub)
    const id = raw.toLowerCase()
        .replace(/[^\w\s-]/g, '')
        .replace(/\s+/g, '-')
        .replace(/-+/g, '-')
        .replace(/^-|-$/g, '');
    return `<h${level} id="${id}">${text}</h${level}>`;
};

const originalLink = renderer.link.bind(renderer);
renderer.link = function(href, title, text) {
    // If the link is an anchor on the current page, prevent default navigation
    if (href.startsWith('#')) {
        return `<a href="${href}" onclick="event.preventDefault(); window.location.hash='${href}'; return false;"${title ? ` title="${title}"` : ''}>${text}</a>`;
    }
    return originalLink(href, title, text);
};

export function parseMarkdown(content) {
    if (!content) return '';
    try {
        return marked.parse(content, { renderer });
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
