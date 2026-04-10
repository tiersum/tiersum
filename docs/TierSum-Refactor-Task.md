# TierSum 搜索与文档详情页重构任务

## 任务概述

将当前 TierSum 的首页搜索和查询结果页进行重构，改为：
1. **章节卡片流展示**（而非文档列表）
2. **AI 回答 + 参考资料左右分栏布局**
3. **热/冷文档差异化详情页处理**

---


## 二、首页搜索改为章节卡片流

### 当前问题
渐进式查询 API `/api/v1/query/progressive` 返回的是**章节级结果**（`QueryItem[]`），但前端错误地按文档去重展示为"文档列表"，导致用户看不到匹配的具体章节内容。

### 修复要求

#### 1. 移除文档聚合逻辑
- **禁止**按 `document_id` 分组去重
- **直接遍历** `data.results` 数组渲染章节卡片

#### 2. 章节卡片信息展示
每个卡片需显示：
- **章节标题**：`item.title`（从 `path` 解析的章节名）
- **所属文档**：从 `item.path`（格式 `doc_id/chapter_path`）解析文档名
- **内容预览**：`item.content` 前 300 字符（章节摘要/片段）
- **相关性**：`item.relevance` 保留两位小数
- **来源标记**：Hot 或 Cold（需根据文档状态判断显示）

#### 3. 排序
按 `relevance` 降序排列，无需额外分组。

#### 4. 卡片交互
- 点击后跳转到文档详情页（区分热/冷文档，见第三部分）
- 悬浮显示完整内容 tooltip（可选）

---

## 三、查询结果页布局（左右分栏）

用户发起搜索后，页面改为**左右分栏布局**：

### 左侧区域（70%宽度）- AI 回答区

**功能**：
- 顶部提示横幅："🤖 以下回答基于右侧检索到的参考资料生成"
- 展示大模型生成的 Markdown 格式回答内容
- 若回答中有引用标记（如 `[^1^]`），点击可联动右侧高亮对应卡片

**数据流**：
```typescript
// 将 results 构建为上下文喂给大模型
const context = results.map((item, index) => `
### [${extractDocName(item.path)}] ${item.title}
**相关性**: ${item.relevance.toFixed(2)}/1.0 | **序号**: ${index + 1}

${item.content}
`).join('\n\n---\n\n');

// System prompt 追加：
// "基于以上参考资料回答用户问题，优先引用相关性高的内容，引用时使用 [^序号^] 格式标注"
```

### 右侧区域（30%宽度）- 参考资料面板

**面板标题**：📚 参考资料（{count} 条）

**内容**：
- 垂直排列的章节卡片列表
- 卡片可点击展开查看完整 `content`
- 空状态："未找到相关参考资料"

**联动逻辑**：
- 左侧回答中的引用标记 `[^n^]` 点击后，右侧自动滚动到第 n 个卡片并高亮
- 右侧卡片 hover 时，提示"点击在回答中查看引用"

---

## 四、详情页差异化处理

章节卡片点击后，根据文档类型（Hot/Cold）差异化跳转和展示：

### 热文档（`status === "hot"`）

**跳转 URL**：
```
/docs/${id}?tier=chapter&path=${encodeURIComponent(item.path)}
```

**详情页行为**：
1. 检测 URL 参数 `tier=chapter` 和 `path`
2. 自动切换至 **Chapter 视图**（三级摘要切换器默认选中 Chapter）
3. 根据 `path` 解析章节路径，**滚动定位到对应章节锚点**（高亮显示）
4. 展示三级摘要切换器：Document → Chapter → Source
5. 显示 Hot Score 和 Query Count 统计

**锚点实现逻辑**：
- 若 `path` 为 `doc_id/scheduler/component`，则查找 ID 或锚点为 `scheduler-component` 的元素
- 使用 `scrollIntoView({ behavior: 'smooth', block: 'center' })` 滚动并高亮

### 冷文档（`status === "cold"`）

**跳转 URL**：
```
/docs/${id}
// 注意：无 path 参数，不做章节锚点跳转
```

**详情页行为**：
1. 直接展示原始 `content`（完整 Markdown 内容）
2. **隐藏三级摘要切换器**（冷文档无 Document/Chapter/Source 结构）
3. 顶部显示提示横幅：
   > ⚡ 此文档为**冷存储**，暂无智能摘要。查询 3 次后将自动升级，或点击下方按钮立即生成。

4. 提供操作按钮：
   - **"🔥 生成摘要"**：调用后端接口触发 LLM 分析（消耗配额，生成三级摘要）
   - **或 "⬆️ 升级热文档"**：加入队列异步处理（若已有后台任务）

5. 升级完成后：
   - 若用户仍在页面，提示刷新
   - 或自动刷新页面展示新摘要

**冷文档特殊情况**：
- 冷文档在首页看到的"章节卡片"实际是 `createColdDocumentChapter` 生成的**关键词片段**（临时提取），非真实章节
- 详情页展示的是完整原始内容，片段位置被淹没在全文中
- 因此**不做锚点定位**，用户需自行在完整文档中查找（或由前端高亮搜索关键词）

---

## 五、UI 组件参考

### 章节卡片组件

```jsx
<Card className="hover:border-blue-500 transition-colors cursor-pointer">
  <CardHeader className="pb-2">
    <div className="flex justify-between items-start">
      <div>
        <Badge variant={isHot ? "default" : "secondary"} className="mb-2">
          {isHot ? "🔥 Hot" : "❄️ Cold"}
        </Badge>
        <CardTitle className="text-lg font-bold">{item.title}</CardTitle>
        <CardDescription className="text-sm text-muted-foreground">
          来自: {extractDocName(item.path)} · 相关性: {item.relevance.toFixed(2)}
        </CardDescription>
      </div>
    </div>
  </CardHeader>
  <CardContent>
    <p className="text-sm text-gray-700 line-clamp-4">
      {item.content}
    </p>
  </CardContent>
  <CardFooter className="flex justify-between">
    <span className="text-xs text-muted-foreground">{item.path}</span>
    <Button variant="ghost" size="sm" asChild>
      <Link href={getDetailUrl(item, docStatus)}>查看详情 →</Link>
    </Button>
  </CardFooter>
</Card>
```

### 工具函数

```typescript
// 从 path 解析文档名
const extractDocName = (path: string) => path.split('/')[0];

// 构建详情页 URL（区分热/冷）
const getDetailUrl = (item: QueryItem, status: 'hot' | 'cold') => {
  if (status === 'hot') {
    return `/docs/${item.id}?tier=chapter&path=${encodeURIComponent(item.path)}`;
  }
  return `/docs/${item.id}`; // 冷文档无锚点
};
```

---

## 六、检查清单

- [ ] 首页直接遍历 `results` 渲染章节卡片，无文档聚合逻辑
- [ ] 章节卡片展示 `content` 内容预览（非仅标题）
- [ ] 章节卡片显示来源标记（Hot/Cold）
- [ ] 查询结果页左右分栏：左侧 AI 回答，右侧参考资料卡片
- [ ] 右侧卡片明确提示"用于基于这些内容回答"
- [ ] 热文档跳转携带 `tier=chapter&path=xxx` 参数
- [ ] 热文档详情页根据 path 滚动到章节锚点
- [ ] 冷文档跳转无 path 参数，详情页不做锚点滚动
- [ ] 冷文档详情页隐藏三级摘要切换器
- [ ] 冷文档详情页顶部显示冷存储提示横幅
- [ ] 冷文档详情页提供"生成摘要"或"升级"按钮
- [ ] 冷文档升级后刷新或提示用户刷新

---

## 七、关键文件路径

- **首页/查询页**：`/web/app/page.tsx` 或 `/web/app/components/QueryInterface.tsx`
- **API 客户端**：`/web/lib/api.ts`（无需修改，类型已定义）
- **文档详情页**：`/web/app/docs/[id]/page.tsx`
- **章节卡片组件**：可新建 `/web/app/components/ChapterCard.tsx`

---

**总结**：首页展示章节卡片流；查询结果左 AI 回答右参考资料；热文档跳章节锚点，冷文档跳全文并提供升级按钮。
