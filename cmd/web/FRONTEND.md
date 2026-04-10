# TierSum Frontend

纯 CDN 前端，无需 Node.js。

## 技术栈

- **Vue 3** - 通过 CDN (unpkg.com)
- **Vue Router 4** - 通过 CDN (unpkg.com)  
- **Tailwind CSS** - 通过 CDN (cdn.tailwindcss.com)
- **DaisyUI** - 通过 CDN (cdn.jsdelivr.net)
- **Marked.js** - 通过 CDN (cdn.jsdelivr.net) 用于 Markdown 渲染

## 文件结构

```
web/
├── index.html    # 主 HTML 入口，加载所有 CDN 资源
├── app.js        # Vue 应用主文件，包含所有组件和路由
└── api.js        # API 客户端 (可选，已合并到 app.js)
```

## 部署

前端文件通过 Go 的 `embed` 功能嵌入到二进制中：

1. 文件位于 `cmd/web/*`
2. 使用 `//go:embed web/*` 嵌入
3. 运行时通过 `StaticFileServer()` 提供静态文件服务

## 构建

```bash
# 只需构建 Go 二进制文件
make build

# 前端文件会自动嵌入，无需额外步骤
```

## 特性

- **Search**: 渐进式搜索，AI 回答 + 参考结果双栏布局
- **Documents**: 文档列表、搜索、创建、删除
- **Tags**: L1 分组 + L2 标签浏览
- **Dark Theme**: Slate 暗色主题
- **Responsive**: 响应式设计，支持移动端

## 路由

- `/` - 搜索页面
- `/docs` - 文档列表
- `/tags` - 标签浏览

使用 Vue Router 的 hash 模式 (`/#/`, `/#/docs`, `/#/tags`)。
