"use client";

import { useState, useCallback } from "react";
import { Search, FileText, BookOpen, ChevronRight, Sparkles } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Separator } from "@/components/ui/separator";
import { api, QueryItem } from "@/lib/api";
import Link from "next/link";

export default function Home() {
  const [query, setQuery] = useState("");
  const [loading, setLoading] = useState(false);
  const [results, setResults] = useState<QueryItem[]>([]);
  const [selectedResult, setSelectedResult] = useState<QueryItem | null>(null);
  const [hasSearched, setHasSearched] = useState(false);

  const handleSearch = useCallback(async () => {
    if (!query.trim()) return;
    
    setLoading(true);
    setHasSearched(true);
    setSelectedResult(null);
    
    try {
      const response = await api.progressiveQuery(query);
      setResults(response.results);
      if (response.results.length > 0) {
        setSelectedResult(response.results[0]);
      }
    } catch (error) {
      console.error("Search failed:", error);
    } finally {
      setLoading(false);
    }
  }, [query]);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter") {
      handleSearch();
    }
  };

  return (
    <div className="min-h-screen bg-slate-950">
      {/* Header */}
      <header className="border-b border-slate-800 bg-slate-950/50 backdrop-blur-sm sticky top-0 z-50">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Sparkles className="w-6 h-6 text-blue-500" />
            <span className="text-xl font-semibold text-slate-100">TierSum</span>
          </div>
          <nav className="flex items-center gap-4">
            <Link href="/docs">
              <Button variant="ghost" size="sm" className="text-slate-400 hover:text-slate-100">
                Documents
              </Button>
            </Link>
            <Link href="/tags">
              <Button variant="ghost" size="sm" className="text-slate-400 hover:text-slate-100">
                Tags
              </Button>
            </Link>
          </nav>
        </div>
      </header>

      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        {/* Search Section */}
        <div className={`transition-all duration-500 ${hasSearched ? 'mb-6' : 'mb-0 mt-32'}`}>
          <div className={`text-center mb-8 ${hasSearched ? 'hidden' : ''}`}>
            <h1 className="text-4xl font-bold text-slate-100 mb-4">
              Search Your Knowledge Base
            </h1>
            <p className="text-slate-400 text-lg max-w-2xl mx-auto">
              Progressive query with hierarchical summarization. 
              Find exactly what you need across all your documents.
            </p>
          </div>

          <div className="max-w-3xl mx-auto relative">
            <div className="relative group">
              <Search className="absolute left-4 top-1/2 -translate-y-1/2 w-5 h-5 text-slate-500 group-focus-within:text-blue-500 transition-colors" />
              <Input
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                onKeyDown={handleKeyDown}
                placeholder="Ask anything about your documents..."
                className="w-full h-14 pl-12 pr-32 text-lg bg-slate-900/50 border-slate-800 text-slate-100 placeholder:text-slate-500 focus:border-blue-500/50 focus:ring-blue-500/20 rounded-xl"
              />
              <Button
                onClick={handleSearch}
                disabled={loading || !query.trim()}
                className="absolute right-2 top-1/2 -translate-y-1/2 bg-blue-600 hover:bg-blue-700 text-white px-6"
              >
                {loading ? "Searching..." : "Search"}
              </Button>
            </div>
          </div>
        </div>

        {/* Results Section */}
        {hasSearched && (
          <div className="grid grid-cols-12 gap-6 mt-8">
            {/* Left Panel - Document List */}
            <div className="col-span-12 lg:col-span-5">
              <Card className="bg-slate-900/50 border-slate-800 h-[calc(100vh-280px)]">
                <CardHeader className="pb-3">
                  <div className="flex items-center justify-between">
                    <h2 className="text-lg font-semibold text-slate-100 flex items-center gap-2">
                      <FileText className="w-5 h-5 text-blue-500" />
                      Results
                    </h2>
                    <Badge variant="secondary" className="bg-slate-800 text-slate-300">
                      {results.length} found
                    </Badge>
                  </div>
                </CardHeader>
                <Separator className="bg-slate-800" />
                <ScrollArea className="h-[calc(100%-80px)]">
                  <div className="p-4 space-y-3">
                    {loading ? (
                      // Loading skeletons
                      Array.from({ length: 5 }).map((_, i) => (
                        <div key={i} className="p-4 rounded-lg bg-slate-800/50 space-y-3">
                          <Skeleton className="h-5 w-3/4 bg-slate-700" />
                          <Skeleton className="h-4 w-full bg-slate-700" />
                          <Skeleton className="h-4 w-2/3 bg-slate-700" />
                        </div>
                      ))
                    ) : results.length === 0 ? (
                      <div className="text-center py-12 text-slate-500">
                        <Search className="w-12 h-12 mx-auto mb-4 opacity-50" />
                        <p>No results found</p>
                      </div>
                    ) : (
                      results.map((result, index) => (
                        <button
                          key={`${result.id}-${index}`}
                          onClick={() => setSelectedResult(result)}
                          className={`w-full text-left p-4 rounded-lg transition-all duration-200 border ${
                            selectedResult?.path === result.path
                              ? "bg-blue-500/10 border-blue-500/50"
                              : "bg-slate-800/30 border-transparent hover:bg-slate-800/60 hover:border-slate-700"
                          }`}
                        >
                          <div className="flex items-start gap-3">
                            <div className="mt-1">
                              {result.tier === 'document' ? (
                                <BookOpen className="w-4 h-4 text-blue-500" />
                              ) : (
                                <FileText className="w-4 h-4 text-emerald-500" />
                              )}
                            </div>
                            <div className="flex-1 min-w-0">
                              <h3 className="font-medium text-slate-200 truncate">
                                {result.title}
                              </h3>
                              <p className="text-sm text-slate-500 mt-1 line-clamp-2">
                                {result.content.substring(0, 120)}...
                              </p>
                              <div className="flex items-center gap-2 mt-2">
                                <Badge variant="outline" className="text-xs border-slate-700 text-slate-400">
                                  {result.tier}
                                </Badge>
                                <span className="text-xs text-slate-600">
                                  {(result.relevance * 100).toFixed(0)}% match
                                </span>
                              </div>
                            </div>
                            <ChevronRight className={`w-4 h-4 mt-1 transition-colors ${
                              selectedResult?.path === result.path
                                ? "text-blue-500"
                                : "text-slate-600"
                            }`} />
                          </div>
                        </button>
                      ))
                    )}
                  </div>
                </ScrollArea>
              </Card>
            </div>

            {/* Right Panel - Detail View */}
            <div className="col-span-12 lg:col-span-7">
              <Card className="bg-slate-900/50 border-slate-800 h-[calc(100vh-280px)]">
                {selectedResult ? (
                  <>
                    <CardHeader className="pb-3">
                      <div className="flex items-center justify-between">
                        <div>
                          <h2 className="text-xl font-semibold text-slate-100">
                            {selectedResult.title}
                          </h2>
                          <div className="flex items-center gap-2 mt-2">
                            <Badge variant="outline" className="border-blue-500/50 text-blue-400">
                              {selectedResult.tier}
                            </Badge>
                            <span className="text-sm text-slate-500">
                              Path: {selectedResult.path}
                            </span>
                          </div>
                        </div>
                        <Link href={`/docs/${selectedResult.id}`}>
                          <Button variant="outline" size="sm" className="border-slate-700 text-slate-300 hover:bg-slate-800">
                            View Full Doc
                          </Button>
                        </Link>
                      </div>
                    </CardHeader>
                    <Separator className="bg-slate-800" />
                    <ScrollArea className="h-[calc(100%-120px)]">
                      <CardContent className="p-6">
                        <div className="prose-dark">
                          <p className="text-slate-300 leading-relaxed whitespace-pre-wrap">
                            {selectedResult.content}
                          </p>
                        </div>
                      </CardContent>
                    </ScrollArea>
                  </>
                ) : (
                  <div className="flex items-center justify-center h-full text-slate-500">
                    <div className="text-center">
                      <BookOpen className="w-16 h-16 mx-auto mb-4 opacity-30" />
                      <p>Select a result to view details</p>
                    </div>
                  </div>
                )}
              </Card>
            </div>
          </div>
        )}
      </main>
    </div>
  );
}
