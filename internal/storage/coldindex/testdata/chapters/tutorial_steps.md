# 快速入门教程

欢迎来到快速入门教程！

## 环境准备

在开始之前，请确保已安装以下工具：

1. Node.js (v18+)
2. Git
3. VS Code

## 创建项目

### 步骤一：初始化项目

打开终端，执行以下命令：

```bash
mkdir my-project
cd my-project
npm init -y
```

### 步骤二：安装依赖

```bash
npm install express
npm install -D nodemon
```

### 步骤三：创建入口文件

创建 `index.js`：

```javascript
const express = require('express');
const app = express();

app.get('/', (req, res) => {
  res.send('Hello World!');
});

app.listen(3000, () => {
  console.log('Server running on port 3000');
});
```

## 运行项目

### 开发模式

```bash
npx nodemon index.js
```

### 生产模式

```bash
node index.js
```

## 常见问题

### 问题一：端口被占用

错误信息：
```
Error: listen EADDRINUSE: address already in use :::3000
```

解决方案：

1. 查找占用端口的进程
2. 终止该进程
3. 或使用其他端口

### 问题二：模块找不到

错误信息：
```
Error: Cannot find module 'express'
```

解决方案：

```bash
npm install
```

## 下一步

恭喜完成快速入门！接下来可以学习：

- 路由配置
- 中间件使用
- 数据库集成
- 部署指南
