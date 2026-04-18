# API 文档示例

本文档展示 API 的基本使用方法。

## 安装

使用 npm 安装：

```bash
npm install my-api
```

## 认证

### 获取 Token

```javascript
const token = await api.auth({
  username: 'admin',
  password: 'secret'
});
```

### 使用 Token

```javascript
api.setHeader('Authorization', `Bearer ${token}`);
```

## 用户接口

### 获取用户列表

```javascript
GET /api/users
```

返回用户列表。

### 获取单个用户

```javascript
GET /api/users/:id
```

返回指定用户信息。

### 创建用户

```javascript
POST /api/users
```

创建新用户。

## 错误处理

### 错误码

| 错误码 | 说明 |
|-------|------|
| 400 | 请求参数错误 |
| 401 | 未认证 |
| 403 | 无权限 |
| 404 | 资源不存在 |
| 500 | 服务器内部错误 |
