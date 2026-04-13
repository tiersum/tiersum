/** End-user product overview: bilingual (English first, then Chinese). */

export const ProductIntroPage = {
    template: `
        <div class="min-h-screen bg-slate-950">
            <main class="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8 py-10 pb-16">
                <p class="text-xs uppercase tracking-widest text-slate-500 mb-2">Product overview</p>
                <h1 class="text-3xl sm:text-4xl font-bold text-slate-100 mb-2">TierSum</h1>
                <p class="text-base text-slate-400 mb-10 max-w-2xl">
                    Hierarchical summaries, <strong class="text-slate-300">chapter-first</strong> retrieval, and—when you need it—<strong class="text-slate-300">up-front AI analysis</strong> feeding a <strong class="text-slate-300">step-by-step</strong> query flow. Details below.
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
                        <p>
                            On the <strong class="text-slate-200">hot path</strong>, AI work runs <strong class="text-slate-200">when documents are ingested</strong>: tags, a document synopsis, and chapter-level blurbs become the
                            <strong class="text-slate-200">pre-shaped layer</strong> that <strong class="text-slate-200">progressive query</strong> reuses—narrowing <em>tags → documents → chapters</em> with LLM scoring at each hop, like skimming an outline before opening the right section.
                            <strong class="text-slate-200">Cold</strong> documents skip most of that upfront cost but stay searched and returned <strong class="text-slate-200">by whole chapters</strong>; ones that see heavy use can <strong class="text-slate-200">promote</strong> to hot when you want the full pre-shaped experience.
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
                            <li><strong class="text-slate-200">Search</strong> — Ask in natural language. <strong class="text-slate-200">Progressive query</strong> walks <em>tags → documents → chapters</em> the way a reader would skim an outline before opening a section: each step uses LLM relevance on top of <strong class="text-slate-200">pre-built summaries and tags</strong> where available, then can synthesize an answer with citations when configured.</li>
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
                            <strong class="text-emerald-300/90">Hot</strong> documents are the ingest-time path described above: chapter-level summaries and tags form the <strong class="text-slate-200">pre-shaped layer</strong> progressive query uses. They count against a configurable hourly quota so costs stay predictable.
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
                            TierSum 用<strong class="text-slate-200">分层摘要</strong>（文档 → 章节 → 原文）保留知识结构，并配合<strong class="text-slate-200">标签与主题</strong>做可解释的导航与筛选，
                            而不是只靠向量「猜」片段边界。
                        </p>
                        <p>
                            在<strong class="text-slate-200">热路径</strong>上，<strong class="text-slate-200">入库时</strong>即可由 AI 写好标签、文档概述与章节级提要，形成后续检索依赖的<strong class="text-slate-200">「已铺好的」语义层</strong>；
                            <strong class="text-slate-200">渐进式查询</strong>在此基础上沿 <strong class="text-slate-200">标签 → 文档 → 章节</strong> 逐步收窄，每一步由大模型打分，像先扫目录再翻到对应小节，路径<strong class="text-slate-200">可解释、可复查</strong>。
                            <strong class="text-slate-200">冷路径</strong>则省去大部分入库时 LLM 成本，仍以<strong class="text-slate-200">整章</strong>建索引与返回；访问频繁时可<strong class="text-slate-200">晋升</strong>为热文档，获得同样的预摘要能力。
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
                            <li><strong class="text-slate-200">搜索</strong> — 用自然语言提问。<strong class="text-slate-200">渐进式查询</strong>沿 <strong class="text-slate-200">标签 → 文档 → 章节</strong> 推进，如同先扫提纲再点开章节：每一步在已有 <strong class="text-slate-200">预摘要与预标签</strong>（热路径文档）之上做相关性判断，必要时再汇总为带引用的回答（视配置而定）。</li>
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
                            <strong class="text-emerald-300/90">热文档</strong>即上文「入库时预写摘要与标签」的路径；渐进式查询主要依托这一层展开，通常受<strong class="text-slate-200">每小时配额</strong>约束以控制成本。
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
