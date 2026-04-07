"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { ArrowLeft, Calendar, Hash, BarChart3, FileText } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Separator } from "@/components/ui/separator";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Skeleton } from "@/components/ui/skeleton";
import { api, Document, SummaryNode } from "@/lib/api";

interface DocumentPageClientProps {
  id: string;
}

export function DocumentPageClient({ id }: DocumentPageClientProps) {
  const [document, setDocument] = useState<Document | null>(null);
  const [summaries, setSummaries] = useState<SummaryNode[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedTier, setSelectedTier] = useState<string>("document");

  useEffect(() => {
    async function loadData() {
      try {
        const [docData, summaryData] = await Promise.all([
          api.getDocument(id),
          api.getDocumentSummaries(id),
        ]);
        setDocument(docData);
        setSummaries(summaryData);
      } catch (error) {
        console.error("Failed to load document:", error);
      } finally {
        setLoading(false);
      }
    }

    loadData();
  }, [id]);

  const getTierSummary = (tier: string) => {
    return summaries.find(s => s.tier === tier)?.content || "No summary available";
  };

  const formatDate = (dateStr: string) => {
    return new Date(dateStr).toLocaleDateString("en-US", {
      year: "numeric",
      month: "short",
      day: "numeric",
    });
  };

  if (loading) {
    return (
      <div className="min-h-screen bg-slate-950">
        <header className="border-b border-slate-800 bg-slate-950/50 backdrop-blur-sm sticky top-0 z-50">
          <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center">
            <Skeleton className="h-10 w-24 bg-slate-800" />
          </div>
        </header>
        <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
          <div className="grid grid-cols-12 gap-6">
            <div className="col-span-12 lg:col-span-8">
              <Skeleton className="h-96 w-full bg-slate-800" />
            </div>
            <div className="col-span-12 lg:col-span-4">
              <Skeleton className="h-64 w-full bg-slate-800" />
            </div>
          </div>
        </main>
      </div>
    );
  }

  if (!document) {
    return (
      <div className="min-h-screen bg-slate-950 flex items-center justify-center">
        <div className="text-center">
          <FileText className="w-16 h-16 mx-auto mb-4 text-slate-600" />
          <h1 className="text-2xl font-bold text-slate-100 mb-2">Document Not Found</h1>
          <p className="text-slate-500 mb-6">The document you&apos;re looking for doesn&apos;t exist.</p>
          <Link href="/">
            <Button className="bg-blue-600 hover:bg-blue-700">
              <ArrowLeft className="w-4 h-4 mr-2" />
              Back to Search
            </Button>
          </Link>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-slate-950">
      {/* Header */}
      <header className="border-b border-slate-800 bg-slate-950/50 backdrop-blur-sm sticky top-0 z-50">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center justify-between">
          <Link href="/">
            <Button variant="ghost" className="text-slate-400 hover:text-slate-100">
              <ArrowLeft className="w-4 h-4 mr-2" />
              Back to Search
            </Button>
          </Link>
        </div>
      </header>

      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        <div className="grid grid-cols-12 gap-6">
          {/* Main Content */}
          <div className="col-span-12 lg:col-span-8">
            <Card className="bg-slate-900/50 border-slate-800">
              <CardHeader className="pb-4">
                <h1 className="text-3xl font-bold text-slate-100 mb-4">
                  {document.title}
                </h1>
                <div className="flex flex-wrap items-center gap-3">
                  <Badge variant="outline" className="border-blue-500/50 text-blue-400">
                    {document.format}
                  </Badge>
                  {document.tags?.map((tag) => (
                    <Badge key={tag} variant="secondary" className="bg-slate-800 text-slate-300">
                      <Hash className="w-3 h-3 mr-1" />
                      {tag}
                    </Badge>
                  ))}
                </div>
              </CardHeader>
              <Separator className="bg-slate-800" />
              <CardContent className="p-6">
                <ScrollArea className="h-[calc(100vh-400px)]">
                  <div className="prose-dark max-w-none">
                    <pre className="whitespace-pre-wrap text-slate-300 leading-relaxed font-sans">
                      {document.content}
                    </pre>
                  </div>
                </ScrollArea>
              </CardContent>
            </Card>
          </div>

          {/* Sidebar */}
          <div className="col-span-12 lg:col-span-4 space-y-6">
            {/* Document Stats */}
            <Card className="bg-slate-900/50 border-slate-800">
              <CardHeader className="pb-3">
                <h3 className="text-lg font-semibold text-slate-100 flex items-center gap-2">
                  <BarChart3 className="w-5 h-5 text-blue-500" />
                  Statistics
                </h3>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="flex items-center justify-between">
                  <span className="text-slate-500">Status</span>
                  <Badge variant={document.status === "indexed" ? "default" : "secondary"}>
                    {document.status}
                  </Badge>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-slate-500">Hot Score</span>
                  <span className="text-slate-200 font-medium">{document.hot_score?.toFixed(2) || "0.00"}</span>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-slate-500">Query Count</span>
                  <span className="text-slate-200 font-medium">{document.query_count || 0}</span>
                </div>
                <Separator className="bg-slate-800" />
                <div className="flex items-center gap-2 text-slate-500">
                  <Calendar className="w-4 h-4" />
                  <span className="text-sm">Created {formatDate(document.created_at)}</span>
                </div>
              </CardContent>
            </Card>

            {/* Summary Tiers */}
            <Card className="bg-slate-900/50 border-slate-800">
              <CardHeader className="pb-3">
                <h3 className="text-lg font-semibold text-slate-100">Summary Tiers</h3>
              </CardHeader>
              <CardContent>
                <div className="space-y-2">
                  {["topic", "document", "chapter", "paragraph"].map((tier) => (
                    <button
                      key={tier}
                      onClick={() => setSelectedTier(tier)}
                      className={`w-full text-left px-4 py-3 rounded-lg transition-all ${
                        selectedTier === tier
                          ? "bg-blue-500/10 border border-blue-500/50"
                          : "bg-slate-800/30 border border-transparent hover:bg-slate-800/60"
                      }`}
                    >
                      <div className="flex items-center justify-between">
                        <span className={`capitalize font-medium ${
                          selectedTier === tier ? "text-blue-400" : "text-slate-300"
                        }`}>
                          {tier} Level
                        </span>
                        {summaries.some(s => s.tier === tier) && (
                          <Badge variant="outline" className="text-xs border-emerald-500/50 text-emerald-400">
                            Available
                          </Badge>
                        )}
                      </div>
                    </button>
                  ))}
                </div>

                {selectedTier && (
                  <div className="mt-4 p-4 rounded-lg bg-slate-800/50 border border-slate-700">
                    <h4 className="text-sm font-medium text-slate-400 mb-2 capitalize">
                      {selectedTier} Summary
                    </h4>
                    <p className="text-sm text-slate-300 leading-relaxed">
                      {getTierSummary(selectedTier)}
                    </p>
                  </div>
                )}
              </CardContent>
            </Card>
          </div>
        </div>
      </main>
    </div>
  );
}
