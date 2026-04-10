"use client";

import { useEffect, useState, useCallback } from "react";
import Link from "next/link";
import { ArrowLeft, FileText, Plus, Calendar, Hash, Sparkles, Search } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Separator } from "@/components/ui/separator";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Skeleton } from "@/components/ui/skeleton";
import { Input } from "@/components/ui/input";
import { Document, api } from "@/lib/api";
import { UploadDialog } from "./_components/upload-dialog";

export default function DocsPage() {
  const [documents, setDocuments] = useState<Document[]>([]);
  const [loading, setLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState("");

  const loadDocuments = useCallback(async () => {
    try {
      setLoading(true);
      const docs = await api.getDocuments();
      setDocuments(docs);
    } catch (error) {
      console.error("Failed to load documents:", error);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadDocuments();
  }, [loadDocuments]);

  const handleUpload = async (doc: {
    title: string;
    content: string;
    format: string;
    tags: string[];
  }) => {
    await api.createDocument(doc);
    // Refresh the list after upload
    await loadDocuments();
  };

  const filteredDocs = documents.filter(doc =>
    doc.title.toLowerCase().includes(searchQuery.toLowerCase()) ||
    doc.tags.some(tag => tag.toLowerCase().includes(searchQuery.toLowerCase()))
  );

  const formatDate = (dateStr: string) => {
    return new Date(dateStr).toLocaleDateString("en-US", {
      year: "numeric",
      month: "short",
      day: "numeric",
    });
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
            <Link href="/">
              <Button variant="ghost" className="text-slate-400 hover:text-slate-100">
                Search
              </Button>
            </Link>
            <Button variant="ghost" className="text-slate-100 bg-slate-800">
              Documents
            </Button>
            <Link href="/tags">
              <Button variant="ghost" className="text-slate-400 hover:text-slate-100">
                Tags
              </Button>
            </Link>
          </nav>
        </div>
      </header>

      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        <div className="flex items-center justify-between mb-8">
          <div>
            <h1 className="text-3xl font-bold text-slate-100 mb-2">Documents</h1>
            <p className="text-slate-400">
              Browse and manage your knowledge base documents
            </p>
          </div>
          <UploadDialog onUpload={handleUpload} />
        </div>

        {/* Search */}
        <div className="mb-6">
          <div className="relative max-w-md">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-500" />
            <Input
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="Search documents..."
              className="pl-10 bg-slate-900/50 border-slate-800 text-slate-100"
            />
          </div>
        </div>

        {/* Documents List */}
        <div className="grid gap-4">
          {loading ? (
            Array.from({ length: 3 }).map((_, i) => (
              <Card key={i} className="bg-slate-900/50 border-slate-800">
                <CardContent className="p-6">
                  <Skeleton className="h-6 w-1/3 bg-slate-800 mb-2" />
                  <Skeleton className="h-4 w-2/3 bg-slate-800" />
                </CardContent>
              </Card>
            ))
          ) : filteredDocs.length === 0 ? (
            <div className="text-center py-12">
              <FileText className="w-16 h-16 mx-auto mb-4 text-slate-600" />
              <h3 className="text-xl font-medium text-slate-300 mb-2">No documents found</h3>
              <p className="text-slate-500 mb-6">Get started by adding your first document</p>
              <UploadDialog onUpload={handleUpload} />
            </div>
          ) : (
            filteredDocs.map((doc) => (
              <Link key={doc.id} href={`/docs/${doc.id}`}>
                <Card className="bg-slate-900/50 border-slate-800 hover:border-slate-700 transition-colors cursor-pointer">
                  <CardContent className="p-6">
                    <div className="flex items-start justify-between">
                      <div className="flex-1">
                        <div className="flex items-center gap-3 mb-2">
                          <FileText className="w-5 h-5 text-blue-500" />
                          <h3 className="text-lg font-semibold text-slate-200">
                            {doc.title}
                          </h3>
                          <Badge variant="outline" className="border-blue-500/50 text-blue-400 text-xs">
                            {doc.format}
                          </Badge>
                          <Badge variant="secondary" className={`text-xs ${
                            doc.status === 'hot' 
                              ? 'bg-orange-500/20 text-orange-400' 
                              : doc.status === 'cold'
                              ? 'bg-blue-500/20 text-blue-400'
                              : 'bg-slate-800 text-slate-400'
                          }`}>
                            {doc.status}
                          </Badge>
                        </div>
                        <p className="text-slate-500 text-sm mb-3 line-clamp-1">
                          {doc.content.substring(0, 100)}...
                        </p>
                        <div className="flex items-center gap-4 text-sm">
                          <div className="flex items-center gap-1 text-slate-500">
                            <Calendar className="w-4 h-4" />
                            {formatDate(doc.created_at)}
                          </div>
                          {doc.tags && doc.tags.length > 0 && (
                            <div className="flex items-center gap-1 text-slate-500">
                              <Hash className="w-4 h-4" />
                              {doc.tags.join(", ")}
                            </div>
                          )}
                        </div>
                      </div>
                      <div className="text-right ml-4">
                        <div className="text-2xl font-bold text-slate-200">
                          {(doc.hot_score || 0).toFixed(2)}
                        </div>
                        <div className="text-xs text-slate-500">hot score</div>
                      </div>
                    </div>
                  </CardContent>
                </Card>
              </Link>
            ))
          )}
        </div>
      </main>
    </div>
  );
}
