/** Landing / home page: hero, features, tech stack, quick start. Static, no backend calls. */

export const LandingPage = {
    template: `
        <div class="min-h-screen bg-slate-950">
            <!-- Hero -->
            <section class="relative overflow-hidden">
                <div class="absolute inset-0 bg-gradient-to-br from-blue-950/40 via-slate-950 to-emerald-950/20"></div>
                <div class="relative max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-20 sm:py-28">
                    <div class="text-center max-w-3xl mx-auto">
                        <div class="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-blue-500/10 border border-blue-500/20 text-blue-300 text-xs font-medium mb-6">
                            <span class="relative flex h-2 w-2">
                                <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-blue-400 opacity-75"></span>
                                <span class="relative inline-flex rounded-full h-2 w-2 bg-blue-500"></span>
                            </span>
                            {{ $t('landingBadge') }}
                        </div>
                        <h1 class="text-4xl sm:text-6xl font-bold text-slate-100 tracking-tight mb-6">
                            {{ $t('landingTitle') }}
                            <span class="bg-gradient-to-r from-blue-400 to-emerald-400 bg-clip-text text-transparent">{{ $t('landingTitleHighlight') }}</span>
                        </h1>
                        <p class="text-lg sm:text-xl text-slate-400 mb-10 leading-relaxed">
                            {{ $t('landingSubtitle') }}
                        </p>
                        <div class="flex flex-col sm:flex-row items-center justify-center gap-4">
                            <router-link to="/login" class="btn btn-primary btn-lg px-8">
                                {{ $t('landingGetStarted') }}
                                <svg class="w-5 h-5 ml-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 7l5 5m0 0l-5 5m5-5H6"/>
                                </svg>
                            </router-link>
                            <router-link to="/about" class="btn btn-outline border-slate-600 btn-lg px-8">
                                {{ $t('landingLearnMore') }}
                            </router-link>
                        </div>
                    </div>
                </div>
            </section>

            <!-- Features Grid -->
            <section class="py-16 sm:py-24 border-t border-slate-800">
                <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
                    <div class="text-center mb-16">
                        <h2 class="text-3xl font-bold text-slate-100 mb-4">{{ $t('landingBuiltFor') }}</h2>
                        <p class="text-slate-400 max-w-2xl mx-auto">
                            {{ $t('landingBuiltForDesc') }}
                        </p>
                    </div>
                    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
                        <div class="card bg-slate-900/50 border border-slate-800 hover:border-slate-700 transition-colors">
                            <div class="card-body">
                                <div class="w-12 h-12 rounded-xl bg-blue-500/10 flex items-center justify-center mb-4">
                                    <svg class="w-6 h-6 text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"/>
                                    </svg>
                                </div>
                                <h3 class="text-lg font-semibold text-slate-100 mb-2">{{ $t('landingFeature1') }}</h3>
                                <p class="text-slate-400 text-sm leading-relaxed">
                                    {{ $t('landingFeature1Desc') }}
                                </p>
                            </div>
                        </div>

                        <div class="card bg-slate-900/50 border border-slate-800 hover:border-slate-700 transition-colors">
                            <div class="card-body">
                                <div class="w-12 h-12 rounded-xl bg-emerald-500/10 flex items-center justify-center mb-4">
                                    <svg class="w-6 h-6 text-emerald-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z"/>
                                    </svg>
                                </div>
                                <h3 class="text-lg font-semibold text-slate-100 mb-2">{{ $t('landingFeature2') }}</h3>
                                <p class="text-slate-400 text-sm leading-relaxed">
                                    {{ $t('landingFeature2Desc') }}
                                </p>
                            </div>
                        </div>

                        <div class="card bg-slate-900/50 border border-slate-800 hover:border-slate-700 transition-colors">
                            <div class="card-body">
                                <div class="w-12 h-12 rounded-xl bg-amber-500/10 flex items-center justify-center mb-4">
                                    <svg class="w-6 h-6 text-amber-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 20l4-16m2 16l4-16M6 9h14M4 15h14"/>
                                    </svg>
                                </div>
                                <h3 class="text-lg font-semibold text-slate-100 mb-2">{{ $t('landingFeature3') }}</h3>
                                <p class="text-slate-400 text-sm leading-relaxed">
                                    {{ $t('landingFeature3Desc') }}
                                </p>
                            </div>
                        </div>

                        <div class="card bg-slate-900/50 border border-slate-800 hover:border-slate-700 transition-colors">
                            <div class="card-body">
                                <div class="w-12 h-12 rounded-xl bg-violet-500/10 flex items-center justify-center mb-4">
                                    <svg class="w-6 h-6 text-violet-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 7h.01M7 3h5c.512 0 1.024.195 1.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A1.994 1.994 0 013 12V7a4 4 0 014-4z"/>
                                    </svg>
                                </div>
                                <h3 class="text-lg font-semibold text-slate-100 mb-2">{{ $t('landingFeature4') }}</h3>
                                <p class="text-slate-400 text-sm leading-relaxed">
                                    {{ $t('landingFeature4Desc') }}
                                </p>
                            </div>
                        </div>

                        <div class="card bg-slate-900/50 border border-slate-800 hover:border-slate-700 transition-colors">
                            <div class="card-body">
                                <div class="w-12 h-12 rounded-xl bg-rose-500/10 flex items-center justify-center mb-4">
                                    <svg class="w-6 h-6 text-rose-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"/>
                                    </svg>
                                </div>
                                <h3 class="text-lg font-semibold text-slate-100 mb-2">{{ $t('landingFeature5') }}</h3>
                                <p class="text-slate-400 text-sm leading-relaxed">
                                    {{ $t('landingFeature5Desc') }}
                                </p>
                            </div>
                        </div>

                        <div class="card bg-slate-900/50 border border-slate-800 hover:border-slate-700 transition-colors">
                            <div class="card-body">
                                <div class="w-12 h-12 rounded-xl bg-cyan-500/10 flex items-center justify-center mb-4">
                                    <svg class="w-6 h-6 text-cyan-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 20l4-16m2 16l4-16M6 9h14M4 15h14"/>
                                    </svg>
                                </div>
                                <h3 class="text-lg font-semibold text-slate-100 mb-2">{{ $t('landingFeature6') }}</h3>
                                <p class="text-slate-400 text-sm leading-relaxed">
                                    {{ $t('landingFeature6Desc') }}
                                </p>
                            </div>
                        </div>
                    </div>
                </div>
            </section>

            <!-- How It Works -->
            <section class="py-16 sm:py-24 border-t border-slate-800">
                <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
                    <div class="text-center mb-16">
                        <h2 class="text-3xl font-bold text-slate-100 mb-4">{{ $t('landingHowItWorks') }}</h2>
                        <p class="text-slate-400 max-w-2xl mx-auto">
                            {{ $t('landingHowItWorksDesc') }}
                        </p>
                    </div>
                    <div class="grid grid-cols-1 md:grid-cols-3 gap-8">
                        <div class="relative">
                            <div class="text-6xl font-bold text-slate-800 mb-4">01</div>
                            <h3 class="text-xl font-semibold text-slate-100 mb-3">{{ $t('landingStep1') }}</h3>
                            <p class="text-slate-400 text-sm leading-relaxed">
                                {{ $t('landingStep1Desc') }}
                            </p>
                        </div>
                        <div class="relative">
                            <div class="text-6xl font-bold text-slate-800 mb-4">02</div>
                            <h3 class="text-xl font-semibold text-slate-100 mb-3">{{ $t('landingStep2') }}</h3>
                            <p class="text-slate-400 text-sm leading-relaxed">
                                {{ $t('landingStep2Desc') }}
                            </p>
                        </div>
                        <div class="relative">
                            <div class="text-6xl font-bold text-slate-800 mb-4">03</div>
                            <h3 class="text-xl font-semibold text-slate-100 mb-3">{{ $t('landingStep3') }}</h3>
                            <p class="text-slate-400 text-sm leading-relaxed">
                                {{ $t('landingStep3Desc') }}
                            </p>
                        </div>
                    </div>
                </div>
            </section>

            <!-- Tech Stack -->
            <section class="py-16 sm:py-24 border-t border-slate-800">
                <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
                    <div class="text-center mb-16">
                        <h2 class="text-3xl font-bold text-slate-100 mb-4">{{ $t('landingTechStack') }}</h2>
                        <p class="text-slate-400 max-w-2xl mx-auto">
                            {{ $t('landingTechStackDesc') }}
                        </p>
                    </div>
                    <div class="grid grid-cols-2 sm:grid-cols-4 gap-4">
                        <div class="card bg-slate-900/50 border border-slate-800">
                            <div class="card-body items-center text-center p-4">
                                <div class="text-2xl mb-2">🚀</div>
                                <div class="font-semibold text-slate-100 text-sm">Go 1.23+</div>
                                <div class="text-xs text-slate-500">Backend</div>
                            </div>
                        </div>
                        <div class="card bg-slate-900/50 border border-slate-800">
                            <div class="card-body items-center text-center p-4">
                                <div class="text-2xl mb-2">⚡</div>
                                <div class="font-semibold text-slate-100 text-sm">Vue 3</div>
                                <div class="text-xs text-slate-500">Frontend</div>
                            </div>
                        </div>
                        <div class="card bg-slate-900/50 border border-slate-800">
                            <div class="card-body items-center text-center p-4">
                                <div class="text-2xl mb-2">🗄️</div>
                                <div class="font-semibold text-slate-100 text-sm">SQLite / PG</div>
                                <div class="text-xs text-slate-500">Database</div>
                            </div>
                        </div>
                        <div class="card bg-slate-900/50 border border-slate-800">
                            <div class="card-body items-center text-center p-4">
                                <div class="text-2xl mb-2">🔍</div>
                                <div class="font-semibold text-slate-100 text-sm">Bleve + HNSW</div>
                                <div class="text-xs text-slate-500">Search</div>
                            </div>
                        </div>
                        <div class="card bg-slate-900/50 border border-slate-800">
                            <div class="card-body items-center text-center p-4">
                                <div class="text-2xl mb-2">🤖</div>
                                <div class="font-semibold text-slate-100 text-sm">OpenAI / Claude</div>
                                <div class="text-xs text-slate-500">LLM</div>
                            </div>
                        </div>
                        <div class="card bg-slate-900/50 border border-slate-800">
                            <div class="card-body items-center text-center p-4">
                                <div class="text-2xl mb-2">📊</div>
                                <div class="font-semibold text-slate-100 text-sm">Prometheus</div>
                                <div class="text-xs text-slate-500">Metrics</div>
                            </div>
                        </div>
                        <div class="card bg-slate-900/50 border border-slate-800">
                            <div class="card-body items-center text-center p-4">
                                <div class="text-2xl mb-2">🔌</div>
                                <div class="font-semibold text-slate-100 text-sm">MCP</div>
                                <div class="text-xs text-slate-500">Protocol</div>
                            </div>
                        </div>
                        <div class="card bg-slate-900/50 border border-slate-800">
                            <div class="card-body items-center text-center p-4">
                                <div class="text-2xl mb-2">🐳</div>
                                <div class="font-semibold text-slate-100 text-sm">Docker</div>
                                <div class="text-xs text-slate-500">Deploy</div>
                            </div>
                        </div>
                    </div>
                </div>
            </section>

            <!-- Quick Start -->
            <section class="py-16 sm:py-24 border-t border-slate-800">
                <div class="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8">
                    <div class="text-center mb-12">
                        <h2 class="text-3xl font-bold text-slate-100 mb-4">{{ $t('landingQuickStart') }}</h2>
                        <p class="text-slate-400">{{ $t('landingQuickStartDesc') }}</p>
                    </div>
                    <div class="space-y-4">
                        <div class="card bg-slate-900/50 border border-slate-800">
                            <div class="card-body">
                                <div class="flex items-center gap-3 mb-3">
                                    <div class="w-8 h-8 rounded-lg bg-blue-500/10 flex items-center justify-center text-blue-400 font-bold text-sm">1</div>
                                    <span class="font-semibold text-slate-100">{{ $t('landingStepClone') }}</span>
                                </div>
                                <pre class="bg-slate-950 rounded-lg p-4 text-sm text-slate-300 overflow-x-auto font-mono">
git clone https://github.com/tiersum/tiersum.git
cd tiersum
cp configs/config.example.yaml configs/config.yaml
# Edit configs/config.yaml and set your OPENAI_API_KEY</pre>
                            </div>
                        </div>

                        <div class="card bg-slate-900/50 border border-slate-800">
                            <div class="card-body">
                                <div class="flex items-center gap-3 mb-3">
                                    <div class="w-8 h-8 rounded-lg bg-blue-500/10 flex items-center justify-center text-blue-400 font-bold text-sm">2</div>
                                    <span class="font-semibold text-slate-100">{{ $t('landingStepRun') }}</span>
                                </div>
                                <pre class="bg-slate-950 rounded-lg p-4 text-sm text-slate-300 overflow-x-auto font-mono">make build
make run</pre>
                            </div>
                        </div>

                        <div class="card bg-slate-900/50 border border-slate-800">
                            <div class="card-body">
                                <div class="flex items-center gap-3 mb-3">
                                    <div class="w-8 h-8 rounded-lg bg-blue-500/10 flex items-center justify-center text-blue-400 font-bold text-sm">3</div>
                                    <span class="font-semibold text-slate-100">{{ $t('landingStepBootstrap') }}</span>
                                </div>
                                <p class="text-slate-400 text-sm">
                                    {{ $t('landingStepBootstrapDesc') }}
                                </p>
                            </div>
                        </div>
                    </div>
                </div>
            </section>

            <!-- CTA -->
            <section class="py-16 sm:py-24 border-t border-slate-800">
                <div class="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8 text-center">
                    <h2 class="text-3xl font-bold text-slate-100 mb-6">{{ $t('landingCTA') }}</h2>
                    <p class="text-slate-400 mb-8 max-w-2xl mx-auto">
                        {{ $t('landingCTADesc') }}
                    </p>
                    <div class="flex flex-col sm:flex-row items-center justify-center gap-4">
                        <router-link to="/login" class="btn btn-primary btn-lg px-8">
                            {{ $t('landingGetStarted') }}
                        </router-link>
                        <a href="https://github.com/tiersum/tiersum" target="_blank" rel="noopener noreferrer" class="btn btn-outline border-slate-600 btn-lg px-8">
                            <svg class="w-5 h-5 mr-2" fill="currentColor" viewBox="0 0 24 24">
                                <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/>
                            </svg>
                            {{ $t('navGitHub') }}
                        </a>
                    </div>
                </div>
            </section>

            <!-- Footer -->
            <footer class="border-t border-slate-800 py-8">
                <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
                    <div class="flex flex-col sm:flex-row items-center justify-between gap-4">
                        <div class="flex items-center gap-2">
                            <svg class="w-5 h-5 text-blue-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z"/>
                            </svg>
                            <span class="text-slate-300 font-semibold">TierSum</span>
                        </div>
                        <div class="flex items-center gap-6 text-sm text-slate-500">
                            <router-link to="/about" class="hover:text-slate-300 transition-colors">{{ $t('navAbout') }}</router-link>
                            <router-link to="/docs" class="hover:text-slate-300 transition-colors">{{ $t('navDocs') }}</router-link>
                            <a href="https://github.com/tiersum/tiersum" target="_blank" rel="noopener noreferrer" class="hover:text-slate-300 transition-colors">{{ $t('navGitHub') }}</a>
                        </div>
                        <div class="text-sm text-slate-600">
                            {{ $t('landingFooterLicense') }}
                        </div>
                    </div>
                </div>
            </footer>
        </div>
    `
};
