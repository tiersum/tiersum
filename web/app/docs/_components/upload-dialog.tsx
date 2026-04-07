"use client";

import { useState } from "react";
import { Upload, X, FileText } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";

interface UploadDialogProps {
  onUpload?: (doc: {
    title: string;
    content: string;
    format: string;
    tags: string[];
  }) => void;
}

export function UploadDialog({ onUpload }: UploadDialogProps) {
  const [open, setOpen] = useState(false);
  const [title, setTitle] = useState("");
  const [content, setContent] = useState("");
  const [format, setFormat] = useState("markdown");
  const [tagInput, setTagInput] = useState("");
  const [tags, setTags] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);

  const handleAddTag = () => {
    if (tagInput.trim() && !tags.includes(tagInput.trim())) {
      setTags([...tags, tagInput.trim()]);
      setTagInput("");
    }
  };

  const handleRemoveTag = (tag: string) => {
    setTags(tags.filter((t) => t !== tag));
  };

  const handleSubmit = async () => {
    if (!title.trim() || !content.trim()) return;

    setLoading(true);
    try {
      await onUpload?.({
        title: title.trim(),
        content: content.trim(),
        format,
        tags,
      });
      setOpen(false);
      setTitle("");
      setContent("");
      setTags([]);
    } catch (error) {
      console.error("Upload failed:", error);
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger
        render={
          <Button className="bg-blue-600 hover:bg-blue-700">
            <Upload className="w-4 h-4 mr-2" />
            Add Document
          </Button>
        }
      />
      <DialogContent className="bg-slate-900 border-slate-800 text-slate-100 max-w-2xl">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <FileText className="w-5 h-5 text-blue-500" />
            Add New Document
          </DialogTitle>
        </DialogHeader>

        <div className="space-y-4 py-4">
          <div className="space-y-2">
            <Label htmlFor="title">Title</Label>
            <Input
              id="title"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="Enter document title..."
              className="bg-slate-800/50 border-slate-700 text-slate-100"
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="format">Format</Label>
            <select
              id="format"
              value={format}
              onChange={(e) => setFormat(e.target.value)}
              className="w-full h-10 px-3 rounded-md bg-slate-800/50 border border-slate-700 text-slate-100"
            >
              <option value="markdown">Markdown</option>
              <option value="text">Plain Text</option>
              <option value="html">HTML</option>
            </select>
          </div>

          <div className="space-y-2">
            <Label htmlFor="content">Content</Label>
            <Textarea
              id="content"
              value={content}
              onChange={(e) => setContent(e.target.value)}
              placeholder="Paste your document content here..."
              className="bg-slate-800/50 border-slate-700 text-slate-100 min-h-[200px]"
            />
          </div>

          <div className="space-y-2">
            <Label>Tags</Label>
            <div className="flex gap-2">
              <Input
                value={tagInput}
                onChange={(e) => setTagInput(e.target.value)}
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
              {tags.map((tag) => (
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
            onClick={() => setOpen(false)}
            className="border-slate-700 text-slate-300 hover:bg-slate-800"
          >
            Cancel
          </Button>
          <Button
            onClick={handleSubmit}
            disabled={loading || !title.trim() || !content.trim()}
            className="bg-blue-600 hover:bg-blue-700"
          >
            {loading ? "Uploading..." : "Upload Document"}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
