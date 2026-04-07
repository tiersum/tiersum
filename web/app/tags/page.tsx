"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { ArrowLeft, Folder, Tag, Hash, ChevronRight, Sparkles } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Separator } from "@/components/ui/separator";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Skeleton } from "@/components/ui/skeleton";
import { api, TagGroup, Tag as TagType } from "@/lib/api";

export default function TagsPage() {
  const [tagGroups, setTagGroups] = useState<TagGroup[]>([]);
  const [tags, setTags] = useState<TagType[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedGroup, setSelectedGroup] = useState<TagGroup | null>(null);

  useEffect(() => {
    async function loadData() {
      try {
        const [groupsData, tagsData] = await Promise.all([
          api.getTagGroups(),
          api.getTags(),
        ]);
        setTagGroups(groupsData);
        setTags(tagsData);
        if (groupsData.length > 0) {
          setSelectedGroup(groupsData[0]);
        }
      } catch (error) {
        console.error("Failed to load tags:", error);
      } finally {
        setLoading(false);
      }
    }

    loadData();
  }, []);

  const getTagsForGroup = (groupId: string) => {
    return tags.filter(tag => tag.group_id === groupId);
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
          <Link href="/">
            <Button variant="ghost" className="text-slate-400 hover:text-slate-100">
              <ArrowLeft className="w-4 h-4 mr-2" />
              Back to Search
            </Button>
          </Link>
        </div>
      </header>

      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        <div className="mb-8">
          <h1 className="text-3xl font-bold text-slate-100 mb-2">Tag Browser</h1>
          <p className="text-slate-400">
            Browse documents by hierarchical tags. L1 groups organize L2 tags into categories.
          </p>
        </div>

        <div className="grid grid-cols-12 gap-6">
          {/* L1 Tag Groups */}
          <div className="col-span-12 lg:col-span-4">
            <Card className="bg-slate-900/50 border-slate-800 h-[calc(100vh-280px)]">
              <CardHeader className="pb-3">
                <h2 className="text-lg font-semibold text-slate-100 flex items-center gap-2">
                  <Folder className="w-5 h-5 text-blue-500" />
                  L1 Groups
                </h2>
              </CardHeader>
              <Separator className="bg-slate-800" />
              <ScrollArea className="h-[calc(100%-80px)]">
                <div className="p-4 space-y-2">
                  {loading ? (
                    Array.from({ length: 5 }).map((_, i) => (
                      <Skeleton key={i} className="h-16 w-full bg-slate-800" />
                    ))
                  ) : tagGroups.length === 0 ? (
                    <div className="text-center py-12 text-slate-500">
                      <Folder className="w-12 h-12 mx-auto mb-4 opacity-50" />
                      <p>No tag groups found</p>
                    </div>
                  ) : (
                    tagGroups.map((group) => (
                      <button
                        key={group.id}
                        onClick={() => setSelectedGroup(group)}
                        className={`w-full text-left p-4 rounded-lg transition-all border ${
                          selectedGroup?.id === group.id
                            ? "bg-blue-500/10 border-blue-500/50"
                            : "bg-slate-800/30 border-transparent hover:bg-slate-800/60 hover:border-slate-700"
                        }`}
                      >
                        <div className="flex items-center justify-between">
                          <div>
                            <h3 className={`font-medium ${
                              selectedGroup?.id === group.id ? "text-blue-400" : "text-slate-200"
                            }`}>
                              {group.name}
                            </h3>
                            {group.description && (
                              <p className="text-sm text-slate-500 mt-1 line-clamp-1">
                                {group.description}
                              </p>
                            )}
                          </div>
                          <Badge variant="secondary" className="bg-slate-800 text-slate-400">
                            {group.tags?.length || 0}
                          </Badge>
                        </div>
                      </button>
                    ))
                  )}
                </div>
              </ScrollArea>
            </Card>
          </div>

          {/* L2 Tags */}
          <div className="col-span-12 lg:col-span-8">
            <Card className="bg-slate-900/50 border-slate-800 h-[calc(100vh-280px)]">
              <CardHeader className="pb-3">
                <div className="flex items-center justify-between">
                  <h2 className="text-lg font-semibold text-slate-100 flex items-center gap-2">
                    <Tag className="w-5 h-5 text-emerald-500" />
                    L2 Tags
                  </h2>
                  {selectedGroup && (
                    <Badge variant="outline" className="border-blue-500/50 text-blue-400">
                      {selectedGroup.name}
                    </Badge>
                  )}
                </div>
              </CardHeader>
              <Separator className="bg-slate-800" />
              <ScrollArea className="h-[calc(100%-80px)]">
                <div className="p-4">
                  {loading ? (
                    <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                      {Array.from({ length: 6 }).map((_, i) => (
                        <Skeleton key={i} className="h-20 w-full bg-slate-800" />
                      ))}
                    </div>
                  ) : !selectedGroup ? (
                    <div className="text-center py-12 text-slate-500">
                      <Folder className="w-12 h-12 mx-auto mb-4 opacity-50" />
                      <p>Select a group to view tags</p>
                    </div>
                  ) : (
                    <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                      {getTagsForGroup(selectedGroup.id).map((tag) => (
                        <Link
                          key={tag.id}
                          href={`/?tag=${encodeURIComponent(tag.name)}`}
                          className="group p-4 rounded-lg bg-slate-800/30 border border-transparent hover:bg-slate-800/60 hover:border-slate-700 transition-all"
                        >
                          <div className="flex items-start justify-between">
                            <div className="flex items-center gap-3">
                              <div className="w-10 h-10 rounded-lg bg-slate-800 flex items-center justify-center group-hover:bg-slate-700 transition-colors">
                                <Hash className="w-5 h-5 text-slate-500 group-hover:text-emerald-500 transition-colors" />
                              </div>
                              <div>
                                <h3 className="font-medium text-slate-200 group-hover:text-slate-100 transition-colors">
                                  {tag.name}
                                </h3>
                                <p className="text-sm text-slate-500">
                                  {tag.document_count} documents
                                </p>
                              </div>
                            </div>
                            <ChevronRight className="w-5 h-5 text-slate-600 group-hover:text-slate-400 transition-colors" />
                          </div>
                        </Link>
                      ))}
                      {getTagsForGroup(selectedGroup.id).length === 0 && (
                        <div className="col-span-2 text-center py-12 text-slate-500">
                          <Tag className="w-12 h-12 mx-auto mb-4 opacity-50" />
                          <p>No tags in this group</p>
                        </div>
                      )}
                    </div>
                  )}
                </div>
              </ScrollArea>
            </Card>
          </div>
        </div>
      </main>
    </div>
  );
}
