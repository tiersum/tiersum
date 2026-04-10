"use client";

import { useEffect, useState, useRef } from "react";
import Link from "next/link";
import { ArrowLeft, Calendar, Hash, BarChart3, FileText, Pencil, Trash2, Flame, Snowflake, Zap, Sparkles } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Separator } from "@/components/ui/separator";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Skeleton } from "@/components/ui/skeleton";
import { api, Document, SummaryNode } from "@/lib/api";
import { useRouter, useSearchParams } from "next/navigation";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import { X, AlertTriangle } from "lucide-react";

interface DocumentPageClientProps {
  id: string;
}

export function DocumentPageClient({ id }: DocumentPageClientProps) {
  const router = useRouter();
  const searchParams = useSearchParams();
  const chapterRef = useRef<HTMLDivElement>(null);
  
  const [document, setDocument] = useState<Document | null>(null);
  const [summaries, setSummaries] = useState<SummaryNode[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedTier, setSelectedTier] = useState<string>("document");
  const [upgrading, setUpgrading] = useState(false);
  
  // Edit dialog state
  const [editOpen, setEditOpen] = useState(false);
  const [editTitle, setEditTitle] = useState("");
  const [editContent, setEditContent] = useState("");
  const [editFormat, setEditFormat] = useState("markdown");
  const [editTags, setEditTags] = useState<string[]>([]);
  const [editTagInput, setEditTagInput] = useState("");
  const [saving, setSaving] = useState(false);
  
  // Delete dialog state
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [deleting, setDeleting] = useState(false);

  // URL 参数
  const tierParam = searchParams.get('tier');
  const pathParam = searchParams.get('path');

  const isHot = document?.status === 'hot';
  const isCold = document?.status === 'cold';

  useEffect(() => {
    async function loadData() {
      try {
        const [docData, summaryData] = await Promise.all([
          api.getDocument(id),
          api.getDocumentSummaries(id),
        ]);
        setDocument(docData);
        setSummaries(summaryData);
        
        // Initialize edit form
        setEditTitle(docData.title);
        setEditContent(docData.content);
        setEditFormat(docData.format);
        setEditTags(docData.tags || []);
        
        // 根据 URL 参数设置默认 tier
        if (tierParam) {
          setSelectedTier(tierParam);
        }
      } catch (error) {
        console.error("Failed to load document:", error);
      } finally {
        setLoading(false);
      }
    }

    loadData();
  }, [id, tierParam]);

  // 滚动到章节锚点
  useEffect(() => {
    if (isHot && pathParam && !loading) {
      // 解析 path，例如 doc_id/scheduler/component -> scheduler-component
      const parts = pathParam.split('/');
      if (parts.length > 1) {
        const chapterPath = parts.slice(1).join('-');
        const element = window.document.getElementById(`chapter-${chapterPath}`);
        if (element) {
          element.scrollIntoView({ behavior: 'smooth', block: 'center' });
          element.classList.add('ring-2', 'ring-blue-500/50');
          setTimeout(() => {
            element.classList.remove('ring-2', 'ring-blue-500/50');
          }, 3000);
        }
      }
    }
  }, [isHot, pathParam, loading, summaries]);

  const handleAddTag = () => {
    if (editTagInput.trim() && !editTags.includes(editTagInput.trim())) {
      setEditTags([...editTags, editTagInput.trim()]);
      setEditTagInput("");
    }
  };

  const handleRemoveTag = (tag: string) => {
    setEditTags(editTags.filter((t) => t !== tag));
  };

  const handleSave = async () => {
    if (!editTitle.trim() || !editContent.trim()) return;
    setSaving(true);
    try {
      await api.updateDocument(id, {
        title: editTitle.trim(),
        content: editContent.trim(),
        format: editFormat,
        tags: editTags,
      });
      const updated = await api.getDocument(id);
      setDocument(updated);
      setEditOpen(false);
    } catch (error) {
      console.error("Failed to update document:", error);
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async () => {
    setDeleting(true);
    try {
      await api.deleteDocument(id);
      router.push("/docs");
    } catch (error) {
      console.error("Failed to delete document:", error);
      setDeleting(false);
    }
  };

  // 冷文档升级
  const handleUpgrade = async () => {
    setUpgrading(true);
    try {
      // 调用 API 触发 LLM 分析
      await api.triggerTagGrouping(); // 暂时使用这个，实际需要新的 API
      
      // 刷新文档状态
      const updated = await api.getDocument(id);
      setDocument(updated);
      
      // 提示用户
      alert('文档升级已触发，请稍后刷新页面查看结果');
    } catch (error) {
      console.error("Failed to upgrade document:", error);
    } finally {
      setUpgrading(false);
    }
  };

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

  // 渲染内容（支持锚点）
  const renderContent = (content: string) => {
    // 如果是热文档且是 chapter 视图，尝试添加锚点
    if (isHot && selectedTier === 'chapter') {
      // 简单处理：将章节标题转换为锚点
      // 实际项目中可能需要更复杂的 Markdown 解析
      return content;
    }
    return content;
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
            <Link href="/docs">
              <Button variant="ghost" className="text-slate-400 hover:text-slate-100">
                Documents
              </Button>
            </Link>
            <Link href="/tags">
              <Button variant="ghost" className="text-slate-400 hover:text-slate-100">
                Tags
              </Button>
            </Link>
          </nav>
        </div>
      </header>

      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        {/* 冷文档提示横幅 */}
        {isCold && (
          <Card className="bg-blue-900/20 border-blue-700/50 mb-6">
            <CardContent className="p-4 flex items-center justify-between">
              <div className="flex items-center gap-3">
                <AlertTriangle className="w-5 h-5 text-blue-400" />
                <div>
                  <p className="text-slate-200 text-sm">
                    ⚡ 此文档为<strong>冷存储</strong>，暂无智能摘要。
                  </p>
                  <p className="text-slate-500 text-xs mt-1">
                    查询 3 次后将自动升级，或点击下方按钮立即生成。
                  </p>
                </div>
              </div>
              <Button 
                onClick={handleUpgrade}
                disabled={upgrading}
                className="bg-blue-600 hover:bg-blue-700"
              >
                <Flame className="w-4 h-4 mr-2" />
                {upgrading ? "处理中..." : "🔥 生成摘要"}
              </Button>
            </CardContent>
          </Card>
        )}

        <div className="grid grid-cols-12 gap-6">
          {/* Main Content */}
          <div className="col-span-12 lg:col-span-8">
            <Card className="bg-slate-900/50 border-slate-800">
              <CardHeader className="pb-4">
                <div className="flex items-start justify-between">
                  <div>
                    <div className="flex items-center gap-2 mb-2">
                      <Badge variant="outline" className="border-blue-500/50 text-blue-400">
                        {document.format}
                      </Badge>
                      <Badge variant={isHot ? "default" : "secondary"} className={isHot ? "bg-orange-500/20 text-orange-400" : "bg-blue-500/20 text-blue-400"}>
                        {isHot ? <Flame className="w-3 h-3 mr-1" /> : <Snowflake className="w-3 h-3 mr-1" />}
                        {document.status}
                      </Badge>
                    </div>
                    <h1 className="text-3xl font-bold text-slate-100 mb-4">
                      {document.title}
                    </h1>
                    <div className="flex flex-wrap items-center gap-3">
                      {document.tags?.map((tag) => (
                        <Badge key={tag} variant="secondary" className="bg-slate-800 text-slate-300">
                          <Hash className="w-3 h-3 mr-1" />
                          {tag}
                        </Badge>
                      ))}
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    <Button
                      variant="outline"
                      onClick={() => setEditOpen(true)}
                      className="border-slate-700 text-slate-300 hover:bg-slate-800"
                    >
                      <Pencil className="w-4 h-4 mr-2" />
                      Edit
                    </Button>
                    <Button
                      variant="outline"
                      onClick={() => setDeleteOpen(true)}
                      className="border-red-700 text-red-400 hover:bg-red-950/30"
                    >
                      <Trash2 className="w-4 h-4 mr-2" />
                      Delete
                    </Button>
                  </div>
                </div>
              </CardHeader>
              <Separator className="bg-slate-800" />
              <CardContent className="p-6">
                <ScrollArea className="h-[calc(100vh-400px)]">
                  <div className="prose-dark max-w-none">
                    {isCold ? (
                      // 冷文档：显示原始 content
                      <pre className="whitespace-pre-wrap text-slate-300 leading-relaxed font-sans">
                        {document.content}
                      </pre>
                    ) : (
                      // 热文档：根据 tier 显示摘要
                      <div className="text-slate-300 leading-relaxed whitespace-pre-wrap">
                        {renderContent(getTierSummary(selectedTier))}
                      </div>
                    )}
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
                  <Badge variant={isHot ? "default" : "secondary"} className={isHot ? "bg-orange-500/20 text-orange-400" : "bg-blue-500/20 text-blue-400"}>
                    {isHot ? <Flame className="w-3 h-3 mr-1" /> : <Snowflake className="w-3 h-3 mr-1" />}
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

            {/* Summary Tiers - 仅热文档显示 */}
            {isHot && (
              <Card className="bg-slate-900/50 border-slate-800">
                <CardHeader className="pb-3">
                  <h3 className="text-lg font-semibold text-slate-100">Summary Tiers</h3>
                </CardHeader>
                <CardContent>
                  <div className="space-y-2">
                    {["document", "chapter", "source"].map((tier) => (
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
            )}

            {/* 冷文档升级提示 */}
            {isCold && (
              <Card className="bg-slate-900/50 border-slate-800">
                <CardHeader className="pb-3">
                  <h3 className="text-lg font-semibold text-slate-100 flex items-center gap-2">
                    <Zap className="w-5 h-5 text-yellow-500" />
                    Upgrade
                  </h3>
                </CardHeader>
                <CardContent>
                  <p className="text-sm text-slate-400 mb-4">
                    升级后可获得：
                  </p>
                  <ul className="text-sm text-slate-500 space-y-2 mb-4">
                    <li className="flex items-center gap-2">
                      <span className="text-emerald-500">✓</span> 三级智能摘要
                    </li>
                    <li className="flex items-center gap-2">
                      <span className="text-emerald-500">✓</span> 章节级检索
                    </li>
                    <li className="flex items-center gap-2">
                      <span className="text-emerald-500">✓</span> 标签分类
                    </li>
                  </ul>
                  <Button 
                    onClick={handleUpgrade}
                    disabled={upgrading}
                    className="w-full bg-orange-600 hover:bg-orange-700"
                  >
                    <Flame className="w-4 h-4 mr-2" />
                    {upgrading ? "处理中..." : "立即升级"}
                  </Button>
                </CardContent>
              </Card>
            )}
          </div>
        </div>
      </main>

      {/* Edit Dialog */}
      <Dialog open={editOpen} onOpenChange={setEditOpen}>
        <DialogContent className="bg-slate-900 border-slate-800 text-slate-100 max-w-5xl max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <Pencil className="w-5 h-5 text-blue-500" />
              Edit Document
            </DialogTitle>
          </DialogHeader>

          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <Label htmlFor="edit-title">Title</Label>
              <Input
                id="edit-title"
                value={editTitle}
                onChange={(e) => setEditTitle(e.target.value)}
                placeholder="Enter document title..."
                className="bg-slate-800/50 border-slate-700 text-slate-100"
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="edit-format">Format</Label>
              <select
                id="edit-format"
                value={editFormat}
                onChange={(e) => setEditFormat(e.target.value)}
                className="w-full h-10 px-3 rounded-md bg-slate-800/50 border border-slate-700 text-slate-100"
              >
                <option value="markdown">Markdown</option>
                <option value="text">Plain Text</option>
                <option value="html">HTML</option>
              </select>
            </div>

            <div className="space-y-2">
              <Label htmlFor="edit-content">Content</Label>
              <Textarea
                id="edit-content"
                value={editContent}
                onChange={(e) => setEditContent(e.target.value)}
                placeholder="Document content..."
                className="bg-slate-800/50 border-slate-700 text-slate-100 min-h-[400px]"
              />
            </div>

            <div className="space-y-2">
              <Label>Tags</Label>
              <div className="flex gap-2">
                <Input
                  value={editTagInput}
                  onChange={(e) => setEditTagInput(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") {
                      e.preventDefault();
                      handleAddTag();
                    }
                  }}
                  placeholder="Add a tag and press Enter..."
                  className="bg-slate-800/50 border-slate-700 text-slate-100"
                />
                <Button
                  type="button"
                  variant="outline"
                  onClick={handleAddTag}
                  className="border-slate-700 text-slate-300 hover:bg-slate-800"
                >
                  Add
                </Button>
              </div>
              <div className="flex flex-wrap gap-2 mt-2">
                {editTags.map((tag) => (
                  <Badge
                    key={tag}
                    variant="secondary"
                    className="bg-slate-800 text-slate-300 cursor-pointer hover:bg-slate-700"
                    onClick={() => handleRemoveTag(tag)}
                  >
                    {tag}
                    <X className="w-3 h-3 ml-1" />
                  </Badge>
                ))}
              </div>
            </div>
          </div>

          <div className="flex justify-end gap-3">
            <Button
              variant="outline"
              onClick={() => setEditOpen(false)}
              className="border-slate-700 text-slate-300 hover:bg-slate-800"
            >
              Cancel
            </Button>
            <Button
              onClick={handleSave}
              disabled={saving || !editTitle.trim() || !editContent.trim()}
              className="bg-blue-600 hover:bg-blue-700"
            >
              {saving ? "Saving..." : "Save Changes"}
            </Button>
          </div>
        </DialogContent>
      </Dialog>

      {/* Delete Dialog */}
      <Dialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <DialogContent className="bg-slate-900 border-slate-800 text-slate-100 max-w-md">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2 text-red-400">
              <Trash2 className="w-5 h-5" />
              Delete Document
            </DialogTitle>
          </DialogHeader>

          <div className="py-4">
            <p className="text-slate-300">
              Are you sure you want to delete <strong>{document?.title}</strong>?
              This action cannot be undone.
            </p>
          </div>

          <div className="flex justify-end gap-3">
            <Button
              variant="outline"
              onClick={() => setDeleteOpen(false)}
              className="border-slate-700 text-slate-300 hover:bg-slate-800"
            >
              Cancel
            </Button>
            <Button
              onClick={handleDelete}
              disabled={deleting}
              className="bg-red-600 hover:bg-red-700"
            >
              {deleting ? "Deleting..." : "Delete"}
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}
