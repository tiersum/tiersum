# TierSum: search home and document detail refactor

## Overview

Refactor the TierSum home search and query results so that:

1. **Chapter cards** are shown in a feed (not a document list).
2. **Split layout**: AI answer on the left, reference materials on the right.
3. **Hot vs cold** documents get different detail-page behavior.

---

## 1. Home search → chapter card feed

### Problem

The progressive query API `POST /api/v1/query/progressive` returns **chapter-level** `QueryItem[]`, but the UI incorrectly deduplicates by document and shows a “document list”, hiding which chapter matched.

### Requirements

#### 1. Remove document aggregation

- **Do not** group or dedupe by `document_id`.
- **Iterate** `data.results` and render one card per item.

#### 2. Each chapter card shows

- **Title**: `item.title` (chapter name from `path` where applicable).
- **Document**: derived from `item.path` (`doc_id/chapter_path`).
- **Preview**: first 300 characters of `item.content` (summary/snippet).
- **Relevance**: `item.relevance` with two decimal places.
- **Source badge**: Hot or Cold from document status.

#### 3. Sorting

Sort by `relevance` descending; no extra grouping.

#### 4. Interaction

- Navigate to the document detail page on click (hot/cold rules in section 3).
- Optional: tooltip with full content on hover.

---

## 2. Query results layout (two columns)

After the user searches, use a **two-column** layout.

### Left (~70%) — AI answer

**Behavior**

- Banner: “This answer was generated from the references on the right.”
- Render the model’s Markdown answer.
- If the answer uses markers like `[^1^]`, clicking them highlights the matching card on the right.

**Data flow**

```typescript
const context = results.map((item, index) => `
### [${extractDocName(item.path)}] ${item.title}
**Relevance**: ${item.relevance.toFixed(2)}/1.0 | **Index**: ${index + 1}

${item.content}
`).join('\n\n---\n\n');

// Append to system prompt, e.g.:
// "Answer using the references above; prefer high-relevance items; cite with [^n^]."
```

### Right (~30%) — References panel

**Title**: References ({count})

**Content**

- Vertical list of chapter cards.
- Cards can expand to show full `content`.
- Empty state: “No references found.”

**Cross-linking**

- Clicking `[^n^]` in the answer scrolls to and highlights the n-th card.
- On card hover, hint: “Click citations in the answer to jump here.”

---

## 3. Detail page: hot vs cold

### Hot documents (`status === "hot"`)

**URL**

```
/docs/${id}?tier=chapter&path=${encodeURIComponent(item.path)}
```

**Behavior**

1. Read `tier=chapter` and `path` from the query string.
2. Open **Chapter** view (three-tier toggle defaults to Chapter).
3. Scroll to the chapter anchor for `path` and highlight it.
4. Show Document → Chapter → Source toggle.
5. Show hot score and query count.

**Anchors**

- For `path` like `doc_id/scheduler/component`, target an element whose id/anchor is `scheduler-component`.
- Use `scrollIntoView({ behavior: 'smooth', block: 'center' })`.

### Cold documents (`status === "cold"`)

**URL**

```
/docs/${id}
// No path query — no chapter anchor navigation
```

**Behavior**

1. Show raw `content` (full Markdown).
2. **Hide** the Document/Chapter/Source toggle (cold docs have no tier structure).
3. Banner:

   > This document is **cold**: no LLM summaries yet. It promotes after 3 queries, or use the button below to generate now.

4. Actions:
   - **Generate summaries**: call backend to run LLM analysis (uses quota, builds three tiers).
   - **Promote to hot**: enqueue async job if supported.

5. After promotion: prompt refresh or auto-reload.

**Cold caveat**

- “Chapter” cards for cold hits may be **keyword snippets** from `createColdDocumentChapter`, not real chapters.
- The detail page shows the full document; snippets are not anchored—users search or highlight keywords in the full text.

---

## 4. UI reference — chapter card

```jsx
<Card className="hover:border-blue-500 transition-colors cursor-pointer">
  <CardHeader className="pb-2">
    <div className="flex justify-between items-start">
      <div>
        <Badge variant={isHot ? "default" : "secondary"} className="mb-2">
          {isHot ? "Hot" : "Cold"}
        </Badge>
        <CardTitle className="text-lg font-bold">{item.title}</CardTitle>
        <CardDescription className="text-sm text-muted-foreground">
          From: {extractDocName(item.path)} · Relevance: {item.relevance.toFixed(2)}
        </CardDescription>
      </div>
    </div>
  </CardHeader>
  <CardContent>
    <p className="text-sm text-gray-700 line-clamp-4">
      {item.content}
    </p>
  </CardContent>
  <CardFooter className="flex justify-between">
    <span className="text-xs text-muted-foreground">{item.path}</span>
    <Button variant="ghost" size="sm" asChild>
      <Link href={getDetailUrl(item, docStatus)}>View detail →</Link>
    </Button>
  </CardFooter>
</Card>
```

### Helpers

```typescript
const extractDocName = (path: string) => path.split('/')[0];

const getDetailUrl = (item: QueryItem, status: 'hot' | 'cold') => {
  if (status === 'hot') {
    return `/docs/${item.id}?tier=chapter&path=${encodeURIComponent(item.path)}`;
  }
  return `/docs/${item.id}`; // cold: no anchor
};
```

---

## 5. Checklist

- [ ] Home renders `results` as chapter cards without document aggregation.
- [ ] Cards show a `content` preview, not only titles.
- [ ] Cards show Hot/Cold badges.
- [ ] Results use a split layout: AI answer left, references right.
- [ ] Right panel copy makes clear these items ground the answer.
- [ ] Hot navigation includes `tier=chapter&path=...`.
- [ ] Hot detail scrolls to the chapter anchor from `path`.
- [ ] Cold navigation omits `path`; no anchor scroll.
- [ ] Cold detail hides the three-tier toggle.
- [ ] Cold detail shows the cold-storage banner.
- [ ] Cold detail offers generate/promote actions.
- [ ] After cold promotion, refresh or prompt the user to refresh.

---

## 6. Likely file paths (Vue 3 CDN UI, embedded)

The shipped UI lives under **`cmd/web/`** (`index.html`, `app.js`, `api.js`). There is no separate root `web/` app in this repo.

---

**Summary**: Chapter-card feed on the home page; split answer + references after search; hot docs deep-link to chapter anchors, cold docs show full text and promotion actions.
