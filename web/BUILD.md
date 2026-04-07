# TierSum Frontend Build Guide

## 构建状态

✅ **构建成功** - 所有页面已生成到 `web/dist/` 目录

## 生成的页面

| 页面 | 文件 | 描述 |
|------|------|------|
| Home | `index.html` | 渐进式搜索主页 |
| Documents | `docs.html` | 文档列表页面 |
| Document Detail | `docs/placeholder.html` | 文档详情页（动态路由） |
| Tags | `tags.html` | 两级标签浏览器 |
| 404 | `404.html` | 错误页面 |

## 快速开始

### 1. 构建前端

```bash
cd web
npm run build
```

### 2. 构建并运行后端

```bash
# 在项目根目录
make build
./build/tiersum
```

### 3. 访问应用

打开浏览器访问 `http://localhost:8080`

## 前端功能

### 首页 (`/`)
- 大型搜索输入框
- 渐进式查询结果展示
- 双栏布局：结果列表 + 详情视图
- 显示层级级别和相关性分数

### 文档列表 (`/docs`)
- 所有文档的列表视图
- 按标题或标签搜索/过滤
- 显示热度分数、查询次数、状态
- 上传文档对话框（UI 已完成，API 待连接）

### 文档详情 (`/docs/[id]`)
- 完整文档内容查看
- 文档统计信息侧边栏
- 摘要层级选择器（主题/文档/章节/段落）
- 标签展示

### 标签浏览器 (`/tags`)
- 两级标签导航
- L1 分组（类别）在左侧
- L2 标签在右侧
- 点击标签进行搜索

## API 集成

前端与 Go 后端在 `http://localhost:8080` 通信：

| 端点 | 方法 | 描述 |
|------|------|------|
| `/api/v1/query/progressive` | POST | 渐进式搜索 |
| `/api/v1/documents` | GET | 列出文档 |
| `/api/v1/documents` | POST | 创建文档 |
| `/api/v1/documents/:id` | GET | 获取文档 |
| `/api/v1/documents/:id/summaries` | GET | 获取文档摘要 |
| `/api/v1/tags` | GET | 列出所有标签 |
| `/api/v1/tags/groups` | GET | 列出标签分组 |
| `/api/v1/tags/group` | POST | 触发标签分组 |

## 配置

编辑 `web/.env.local`：

```
NEXT_PUBLIC_API_URL=http://localhost:8080
```

## 设计系统

### 颜色
- 背景：`slate-950` (#020617)
- 卡片：`slate-900/50`，边框 `slate-800`
- 主色：`blue-500/600`
- 文字主色：`slate-100`
- 文字次色：`slate-400`
- 文字弱化：`slate-500`

### 组件
- 卡片：微妙的边框和悬停效果
- 徽章：用于状态/标签，使用 outline 变体
- 按钮：导航使用 ghost 变体
- 滚动区域：用于长内容
- 骨架屏：加载状态

## 注意事项

1. **静态导出**：项目配置为静态导出 (`output: 'export'`)，生成可由 Go 后端服务的静态 HTML 文件

2. **动态路由**：文档详情页使用 `generateStaticParams` 进行静态导出兼容

3. **API 响应格式**：前端期望 API 响应格式为 `{ data: ... }` 或直接数据对象

4. **CORS**：确保后端允许前端域名的跨域请求（开发时为 `localhost:3000`，生产时为 `localhost:8080`）
