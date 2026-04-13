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
                        <p class="rounded-lg border border-slate-700/80 bg-slate-900/60 px-4 py-3 text-slate-200">
                            <strong class="text-blue-300">Chapter-first, hot or cold.</strong>
                            For <em>both</em> paths, TierSum treats <strong>Markdown sections (chapters)</strong> as the working unit — aligned with headings and document structure — instead of blind fixed-size fragments.
                            Summaries, progressive narrowing, and cold search all respect those boundaries so <strong>meaning stays intact end-to-end</strong>.
                        </p>
                    </section>

                    <section class="space-y-4">
                        <h2 class="text-xl font-semibold text-slate-100 border-b border-slate-800 pb-2">What you can do</h2>
                        <ul class="list-disc pl-5 space-y-2 marker:text-blue-500/80">
                            <li><strong class="text-slate-200">Search</strong> — Ask in natural language. The app narrows <em>tags → documents → chapters</em> with relevance scoring at each step, then can synthesize an answer with citations when configured.</li>
                            <li><strong class="text-slate-200">Documents</strong> — Ingest Markdown (and more over time). Hot docs get LLM summaries and tags per <strong class="text-slate-200">chapter</strong>; cold docs are indexed and retrieved the same way — <strong class="text-slate-200">by chapter</strong> — so every tier keeps coherent sections, not shredded text.</li>
                            <li><strong class="text-slate-200">Tags</strong> — Browse a shared catalog of tags grouped into <em>topics</em> (themes). Regroup refreshes those themes from your catalog so navigation stays meaningful as the library grows.</li>
                        </ul>
                    </section>

                    <section class="space-y-4">
                        <h2 class="text-xl font-semibold text-slate-100 border-b border-slate-800 pb-2">Hot vs cold (plain language)</h2>
                        <p class="text-slate-200">
                            The difference is <em>how much LLM work runs on ingest</em> — not the grain of your knowledge.
                            <strong class="text-slate-100">Hot and cold both stay chapter-centric:</strong> retrieval, ranking, and what you read back are built on <strong>whole markdown chapters</strong>, preserving semantic integrity whether the doc is fully analyzed or cost-optimized.
                        </p>
                        <p>
                            <strong class="text-emerald-300/90">Hot</strong> documents add richer treatment: LLM summaries and tags at the chapter level, plus progressive search that filters down to those chapters. They count against a configurable hourly quota so costs stay predictable.
                        </p>
                        <p>
                            <strong class="text-sky-300/90">Cold</strong> documents skip heavy LLM work on ingest but use the same <strong class="text-slate-200">chapter-sized</strong> index for BM25 and optional semantic ranking — hits return <em>entire sections</em>, not arbitrary snippets. Frequently used cold docs can be promoted toward hot automatically.
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
                        <p class="rounded-lg border border-slate-700/80 bg-slate-900/60 px-4 py-3 text-slate-200">
                            <strong class="text-blue-300">冷热一致：以章节为单元。</strong>
                            无论走「热」还是「冷」，系统都以 <strong>Markdown 章节</strong>（按标题划分的自然段）作为工作与检索粒度，而不是固定长度的随机碎片。
                            摘要、渐进式收窄与冷索引检索都沿这一边界展开，<strong>保证语义从头到尾完整、可理解</strong>。
                        </p>
                    </section>

                    <section class="space-y-4">
                        <h2 class="text-xl font-semibold text-slate-100 border-b border-slate-800 pb-2">能做什么</h2>
                        <ul class="list-disc pl-5 space-y-2 marker:text-blue-500/80">
                            <li><strong class="text-slate-200">搜索</strong> — 用自然语言提问；系统在<strong class="text-slate-200">标签 → 文档 → 章节</strong>上逐级用相关性过滤，必要时可生成带引用的回答（视配置而定）。</li>
                            <li><strong class="text-slate-200">文档</strong> — 导入 Markdown 等；热路径按 <strong class="text-slate-200">章节</strong> 生成摘要与标签；冷路径同样按 <strong class="text-slate-200">章节</strong> 建索引与返回命中，冷热都以章节为边界，避免正文被无意义切碎。</li>
                            <li><strong class="text-slate-200">标签</strong> — 浏览全库标签，并按 <strong class="text-slate-200">主题</strong> 分组浏览；「重归类」会用 LLM 根据当前标签目录刷新主题，便于随库增长维护导航。</li>
                        </ul>
                    </section>

                    <section class="space-y-4">
                        <h2 class="text-xl font-semibold text-slate-100 border-b border-slate-800 pb-2">热文档与冷文档（通俗理解）</h2>
                        <p class="text-slate-200">
                            差别主要在<strong class="text-slate-100">入库时做多少 LLM 分析</strong>，而不是把知识切成不同大小的「块」。
                            <strong class="text-slate-100">热与冷都以章节为中心：</strong>检索、排序与展示都建立在 <strong>完整 Markdown 章节</strong> 上，沿标题结构保持语义边界，避免任意碎片破坏语境。
                        </p>
                        <p>
                            <strong class="text-emerald-300/90">热文档</strong>在章节粒度上增加 LLM 摘要与打标签，并支持 progressive 查询逐步收窄到相关章节；通常受<strong class="text-slate-200">每小时配额</strong>约束以控制成本。
                        </p>
                        <p>
                            <strong class="text-sky-300/90">冷文档</strong>入库时几乎不做完整 LLM 分析，但同样按 <strong class="text-slate-200">章节</strong> 建索引，用 <strong class="text-slate-200">BM25 + 向量</strong> 混合检索，命中返回<strong class="text-slate-200">整章正文</strong>；查询次数达到阈值后可自动向「热」晋升。
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
