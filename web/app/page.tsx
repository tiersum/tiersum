"use client";

import { useState, useCallback, useRef, useEffect } from "react";
import { Search, FileText, Sparkles, Flame, Snowflake, ChevronRight, BookOpen } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle, CardDescription, CardFooter } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Separator } from "@/components/ui/separator";
import { api, QueryItem } from "@/lib/api";
import Link from "next/link";
import ReactMarkdown from "react-markdown";

// 从 path 解析文档名
const extractDocName = (path: string) => {
  const parts = path.split('/');
  return parts[0] || path;
};

// 从 path 解析章节路径
const extractChapterPath = (path: string) => {
  const parts = path.split('/');
  if (parts.length > 1) {
    return parts.slice(1).join('/');
  }
  return '';
};

// 构建详情页 URL（区分热/冷）
const getDetailUrl = (item: QueryItem & { docStatus?: 'hot' | 'cold' }) => {
  if (item.docStatus === 'hot') {
    return `/docs/${item.id}?tier=chapter&path=${encodeURIComponent(item.path)}`;
  }
  return `/docs/${item.id}`;
};

// 章节卡片组件
function ChapterCard({ 
  item, 
  index, 
  isHighlighted,
  onHighlight 
}: { 
  item: QueryItem & { docStatus?: 'hot' | 'cold' };
  index: number;
  isHighlighted?: boolean;
  onHighlight?: (index: number) => void;
}) {
  const docName = extractDocName(item.path);
  const chapterPath = extractChapterPath(item.path);
  const isHot = item.docStatus === 'hot';
  
  return (
    <Card 
      id={`ref-${index}`}
      className={`bg-slate-900/50 border-slate-800 hover:border-blue-500/50 transition-all cursor-pointer ${
        isHighlighted ? 'ring-2 ring-blue-500/50 border-blue-500/50' : ''
      }`}
      onClick={() => onHighlight?.(index)}
    >
      <CardHeader className="pb-2">
        <div className="flex justify-between items-start">
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2 mb-2">
              <Badge variant={isHot ? "default" : "secondary"} className={isHot ? "bg-orange-500/20 text-orange-400" : "bg-blue-500/20 text-blue-400"}>
                {isHot ? <Flame className="w-3 h-3 mr-1" /> : <Snowflake className="w-3 h-3 mr-1" />}
                {isHot ? "Hot" : "Cold"}
              </Badge>
              <span className="text-xs text-slate-500">#{index + 1}</span>
            </div>
            <CardTitle className="text-base font-semibold text-slate-200 line-clamp-2">
              {item.title}
            </CardTitle>
            <CardDescription className="text-xs text-slate-500 mt-1">
              来自: {docName} {chapterPath && `· ${chapterPath}`}
            </CardDescription>
          </div>
          <Badge variant="outline" className="text-xs border-slate-700 text-slate-400 ml-2">
            {(item.relevance * 100).toFixed(0)}%
          </Badge>
        </div>
      </CardHeader>
      <CardContent>
        <p className="text-sm text-slate-400 line-clamp-4">
          {item.content.substring(0, 300)}
          {item.content.length > 300 && "..."}
        </p>
      </CardContent>
      <CardFooter className="flex justify-between pt-2">
        <span className="text-xs text-slate-600 truncate max-w-[200px]">{item.path}</span>
        <Link href={getDetailUrl(item)} className="text-sm text-slate-400 hover:text-slate-200 flex items-center">
          查看详情 <ChevronRight className="w-4 h-4 ml-1" />
        </Link>
      </CardFooter>
    </Card>
  );
}

export default function Home() {
  const [query, setQuery] = useState("");
  const [loading, setLoading] = useState(false);
  const [results, setResults] = useState<(QueryItem & { docStatus?: 'hot' | 'cold' })[]>([]);
  const [hasSearched, setHasSearched] = useState(false);
  const [aiAnswer, setAiAnswer] = useState<string>("");
  const [aiLoading, setAiLoading] = useState(false);
  const [highlightedRef, setHighlightedRef] = useState<number | null>(null);
  const answerRef = useRef<HTMLDivElement>(null);

  // 构建上下文用于 AI 回答
  const buildContext = (results: QueryItem[]) => {
    return results.map((item, index) => `
### [${extractDocName(item.path)}] ${item.title}
**相关性**: ${item.relevance.toFixed(2)}/1.0 | **序号**: ${index + 1}

${item.content.substring(0, 1000)}
`).join('\n\n---\n\n');
  };

  // 生成 AI 回答
  const generateAiAnswer = async (question: string, context: string) => {
    // 这里应该调用后端 API 生成 AI 回答
    // 暂时模拟一个回答，实际项目中应该调用 LLM API
    const mockAnswer = `根据检索到的 ${results.length} 条参考资料，我来回答您的问题：

**关于 "${question}" 的解答：**

基于检索结果，我发现以下关键信息：

${results.slice(0, 3).map((r, i) => `- ${r.title} (相关性: ${(r.relevance * 100).toFixed(0)}%) [^${i + 1}^]`).join('\n')}

${results[0]?.content.substring(0, 200)}...

> 注：以上为基于参考资料的总结，详细内容请查看右侧参考卡片。

**引用说明：**
- [^1^] 相关性最高的文档章节
- [^2^] 次要参考内容  
- [^3^] 补充参考资料`;

    return mockAnswer;
  };

  const handleSearch = useCallback(async () => {
    if (!query.trim()) return;
    
    setLoading(true);
    setAiLoading(true);
    setHasSearched(true);
    setAiAnswer("");
    
    try {
      const response = await api.progressiveQuery(query);
      
      // 为每个结果添加文档状态标记（需要从后端获取文档状态）
      // 暂时假设所有文档都是 hot，实际需要调用 API 获取
      const resultsWithStatus = response.results.map(r => ({
        ...r,
        docStatus: 'hot' as 'hot' | 'cold'
      }));
      
      setResults(resultsWithStatus);
      
      // 生成 AI 回答
      const context = buildContext(response.results);
      const answer = await generateAiAnswer(query, context);
      setAiAnswer(answer);
    } catch (error) {
      console.error("Search failed:", error);
    } finally {
      setLoading(false);
      setAiLoading(false);
    }
  }, [query]);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter") {
      handleSearch();
    }
  };

  // 处理引用点击
  const handleCitationClick = (refNum: number) => {
    setHighlightedRef(refNum);
    const element = document.getElementById(`ref-${refNum}`);
    if (element) {
      element.scrollIntoView({ behavior: 'smooth', block: 'center' });
    }
  };

  // 渲染 AI 回答中的引用标记
  const renderAiAnswer = (content: string) => {
    // 替换 [^n^] 为可点击的链接
    const parts = content.split(/(\[\^\d+\^\])/g);
    return parts.map((part, i) => {
      const match = part.match(/\[\^(\d+)\^\]/);
      if (match) {
        const refNum = parseInt(match[1]) - 1;
        return (
          <sup key={i}>
            <Button
              variant="link"
              size="sm"
              className="px-1 py-0 h-auto text-blue-400 hover:text-blue-300"
              onClick={() => handleCitationClick(refNum)}
            >
              [{match[1]}]
            </Button>
          </sup>
        );
      }
      return <span key={i}>{part}</span>;
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
            <Button variant="ghost" className="text-slate-100 bg-slate-800">
              Search
            </Button>
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

      <main className="max-w-[1600px] mx-auto px-4 sm:px-6 lg:px-8 py-8">
        {/* Search Section */}
        <div className={`transition-all duration-500 ${hasSearched ? 'mb-6' : 'mb-0 mt-32'}`}>
          <div className={`text-center mb-8 ${hasSearched ? 'hidden' : ''}`}>
            <h1 className="text-4xl font-bold text-slate-100 mb-4">
              Search Your Knowledge Base
            </h1>
            <p className="text-slate-400 text-lg max-w-2xl mx-auto">
              AI-powered search with hierarchical summarization. 
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

        {/* Results Section - 左右分栏 */}
        {hasSearched && (
          <div className="grid grid-cols-12 gap-6 mt-8">
            {/* Left Panel - AI Answer (70%) */}
            <div className="col-span-12 lg:col-span-8">
              <Card className="bg-slate-900/50 border-slate-800 h-[calc(100vh-280px)]">
                <CardHeader className="pb-3 border-b border-slate-800">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <BookOpen className="w-5 h-5 text-emerald-500" />
                      <h2 className="text-lg font-semibold text-slate-100">AI 回答</h2>
                    </div>
                    <Badge variant="outline" className="text-xs border-emerald-500/50 text-emerald-400">
                      基于 {results.length} 条参考资料
                    </Badge>
                  </div>
                  <p className="text-xs text-slate-500 mt-2">
                    🤖 以下回答基于右侧检索到的参考资料生成
                  </p>
                </CardHeader>
                <ScrollArea className="h-[calc(100%-100px)]">
                  <CardContent className="p-6">
                    {aiLoading ? (
                      <div className="space-y-4">
                        <Skeleton className="h-4 w-full bg-slate-800" />
                        <Skeleton className="h-4 w-5/6 bg-slate-800" />
                        <Skeleton className="h-4 w-4/6 bg-slate-800" />
                        <Skeleton className="h-20 w-full bg-slate-800 mt-4" />
                      </div>
                    ) : aiAnswer ? (
                      <div className="prose-dark prose-sm max-w-none">
                        <div className="text-slate-300 leading-relaxed whitespace-pre-wrap">
                          {renderAiAnswer(aiAnswer)}
                        </div>
                      </div>
                    ) : (
                      <div className="text-center py-12 text-slate-500">
                        <p>AI 回答生成中...</p>
                      </div>
                    )}
                  </CardContent>
                </ScrollArea>
              </Card>
            </div>

            {/* Right Panel - References (30%) */}
            <div className="col-span-12 lg:col-span-4">
              <Card className="bg-slate-900/50 border-slate-800 h-[calc(100vh-280px)]">
                <CardHeader className="pb-3 border-b border-slate-800">
                  <div className="flex items-center justify-between">
                    <h2 className="text-lg font-semibold text-slate-100 flex items-center gap-2">
                      <FileText className="w-5 h-5 text-blue-500" />
                      参考资料
                    </h2>
                    <Badge variant="secondary" className="bg-slate-800 text-slate-300">
                      {results.length} 条
                    </Badge>
                  </div>
                  <p className="text-xs text-slate-500 mt-2">
                    用于基于这些内容回答您的问题
                  </p>
                </CardHeader>
                <ScrollArea className="h-[calc(100%-100px)]">
                  <div className="p-4 space-y-4">
                    {loading ? (
                      Array.from({ length: 5 }).map((_, i) => (
                        <div key={i} className="p-4 rounded-lg bg-slate-800/50 space-y-3">
                          <Skeleton className="h-5 w-3/4 bg-slate-700" />
                          <Skeleton className="h-4 w-full bg-slate-700" />
                          <Skeleton className="h-4 w-2/3 bg-slate-700" />
                        </div>
                      ))
                    ) : results.length === 0 ? (
                      <div className="text-center py-12 text-slate-500">
                        <FileText className="w-12 h-12 mx-auto mb-4 opacity-50" />
                        <p>未找到相关参考资料</p>
                      </div>
                    ) : (
                      results.map((result, index) => (
                        <ChapterCard
                          key={`${result.id}-${index}`}
                          item={result}
                          index={index}
                          isHighlighted={highlightedRef === index}
                          onHighlight={setHighlightedRef}
                        />
                      ))
                    )}
                  </div>
                </ScrollArea>
              </Card>
            </div>
          </div>
        )}
      </main>
    </div>
  );
}
