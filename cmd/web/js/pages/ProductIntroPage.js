/** End-user product overview: bilingual (English first, then Chinese). */

export const ProductIntroPage = {
    template: `
        <div class="min-h-screen bg-slate-950">
            <main class="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8 py-10 pb-16">
                <p class="text-xs uppercase tracking-widest text-slate-500 mb-2">Product overview</p>
                <h1 class="text-3xl sm:text-4xl font-bold text-slate-100 mb-2">TierSum</h1>
                <p class="text-lg text-blue-300/90 mb-10">
                    Hierarchical summary knowledge base — find answers without chopping docs into anonymous vector chunks.
                </p>

                <article class="space-y-10 text-slate-300 leading-relaxed">
                    <section class="space-y-4">
                        <h2 class="text-xl font-semibold text-slate-100 border-b border-slate-800 pb-2">Why TierSum</h2>
                        <p>
                            Many retrieval systems split text into small overlapping chunks and rely mainly on similarity search.
                            That can blur structure and lose context. TierSum keeps a <strong class="text-slate-200">clear hierarchy</strong>:
                            document overview, chapter-level summaries, and original Markdown — plus a <strong class="text-slate-200">tag and topic layer</strong>
                            so you navigate knowledge the way humans organize it, not the way embeddings shard it.
                        </p>
                    </section>

                    <section class="space-y-4">
                        <h2 class="text-xl font-semibold text-slate-100 border-b border-slate-800 pb-2">What you can do</h2>
                        <ul class="list-disc pl-5 space-y-2 marker:text-blue-500/80">
                            <li><strong class="text-slate-200">Search</strong> — Ask in natural language. The app narrows <em>tags → documents → chapters</em> with relevance scoring at each step, then can synthesize an answer with citations when configured.</li>
                            <li><strong class="text-slate-200">Documents</strong> — Ingest Markdown (and more over time). Important material can receive full LLM summaries and tags; long-tail content can stay “cold” with fast keyword + semantic search over chapters.</li>
                            <li><strong class="text-slate-200">Tags</strong> — Browse a shared catalog of tags grouped into <em>topics</em> (themes). Regroup refreshes those themes from your catalog so navigation stays meaningful as the library grows.</li>
                        </ul>
                    </section>

                    <section class="space-y-4">
                        <h2 class="text-xl font-semibold text-slate-100 border-b border-slate-800 pb-2">Hot vs cold (plain language)</h2>
                        <p>
                            <strong class="text-emerald-300/90">Hot</strong> documents get richer treatment: summaries, chapter breakdowns, and tags for progressive search. They count against a configurable hourly quota so costs stay predictable.
                        </p>
                        <p>
                            <strong class="text-sky-300/90">Cold</strong> documents skip heavy LLM work on ingest but stay searchable with BM25 plus optional semantic ranking over <em>whole chapters</em> — you see real sections, not mystery snippets. Frequently used cold docs can be promoted toward hot automatically.
                        </p>
                    </section>

                    <section class="space-y-4">
                        <h2 class="text-xl font-semibold text-slate-100 border-b border-slate-800 pb-2">Who it is for</h2>
                        <p>
                            Teams that live in Markdown: internal runbooks, architecture notes, support playbooks, research memos, and agent-facing knowledge.
                            The same instance exposes a browser UI and programmatic access (REST and MCP) so humans and automation share one source of truth.
                        </p>
                    </section>
                </article>

                <div class="my-14 flex items-center gap-4" aria-hidden="true">
                    <div class="flex-1 h-px bg-gradient-to-r from-transparent via-slate-600 to-transparent"></div>
                    <span class="text-sm font-medium text-slate-500 shrink-0">中文</span>
                    <div class="flex-1 h-px bg-gradient-to-r from-transparent via-slate-600 to-transparent"></div>
                </div>

                <article class="space-y-10 text-slate-300 leading-relaxed">
                    <section class="space-y-4">
                        <h2 class="text-xl font-semibold text-slate-100 border-b border-slate-800 pb-2">为何选择 TierSum</h2>
                        <p>
                            常见检索会把长文切成大量小块再做相似度匹配，容易<strong class="text-slate-200">丢失结构与语境</strong>。
                            TierSum 用<strong class="text-slate-200">分层摘要</strong>（文档 → 章节 → 原文）保留知识结构，并配合<strong class="text-slate-200">标签与主题（topics）</strong>做可解释的导航与筛选，
                            而不是只靠向量「猜」片段边界。
                        </p>
                    </section>

                    <section class="space-y-4">
                        <h2 class="text-xl font-semibold text-slate-100 border-b border-slate-800 pb-2">能做什么</h2>
                        <ul class="list-disc pl-5 space-y-2 marker:text-blue-500/80">
                            <li><strong class="text-slate-200">搜索</strong> — 用自然语言提问；系统在<strong class="text-slate-200">标签 → 文档 → 章节</strong>上逐级用相关性过滤，必要时可生成带引用的回答（视配置而定）。</li>
                            <li><strong class="text-slate-200">文档</strong> — 导入 Markdown 等；重要内容可走「热」路径获得完整摘要与标签；海量或次要内容可走「冷」路径，以章节级混合检索为主，成本更低。</li>
                            <li><strong class="text-slate-200">标签</strong> — 浏览全库标签，并按 <strong class="text-slate-200">主题</strong> 分组浏览；「重归类」会用 LLM 根据当前标签目录刷新主题，便于随库增长维护导航。</li>
                        </ul>
                    </section>

                    <section class="space-y-4">
                        <h2 class="text-xl font-semibold text-slate-100 border-b border-slate-800 pb-2">热文档与冷文档（通俗理解）</h2>
                        <p>
                            <strong class="text-emerald-300/90">热文档</strong>在入库时可做更完整的 LLM 分析与摘要、章节拆分和打标签，适合经常被引用、需要精细 progressive 查询的核心资料；通常受<strong class="text-slate-200">每小时配额</strong>约束以控制成本。
                        </p>
                        <p>
                            <strong class="text-sky-300/90">冷文档</strong>入库时几乎不做完整 LLM 分析，但会对 Markdown 章节建索引，支持 <strong class="text-slate-200">BM25 + 向量</strong> 混合检索，命中返回<strong class="text-slate-200">整章正文</strong>而非零碎片段；查询次数达到阈值后可自动向「热」晋升。
                        </p>
                    </section>

                    <section class="space-y-4">
                        <h2 class="text-xl font-semibold text-slate-100 border-b border-slate-800 pb-2">适合谁</h2>
                        <p>
                            以 Markdown 为中心的知识团队：内部手册、架构说明、运维笔记、研究纪要，以及需要 <strong class="text-slate-200">REST / MCP</strong> 对接的智能体与工作流。
                            浏览器界面与程序化接口共用同一套数据，减少「人看的知识库」和「机器用的接口」分裂。
                        </p>
                    </section>
                </article>
            </main>
        </div>
    `
};
