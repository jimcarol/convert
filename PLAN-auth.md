# API 安全认证方案：JWT + 主密码 + 双模式存储

## Context

当前所有 API（包括密码管理、笔记等敏感数据）完全公开，任何人都可以访问。需要添加认证机制保护所有端点。

采用**单主密码 + JWT Cookie** 方案，同时前端支持**双模式存储**：
- **已登录用户**：数据通过 API 远程存储到服务器
- **未登录用户**：数据保存在浏览器 localStorage，完全本地使用

## 方案优势

- 简单：只需一个密码，无需用户管理系统
- 兼容：Cookie 自动发送，passwordx Next.js 打包文件无需修改
- 灵活：未登录也能使用，数据存本地 localStorage
- 安全：HttpOnly Cookie 防 XSS，SameSite=Lax 防 CSRF

## 实现步骤

### 1. 添加依赖

```bash
go get github.com/golang-jwt/jwt/v5
```

### 2. 新建 `middleware/auth.go`

- `AuthRequired(jwtSecret string) gin.HandlerFunc`
- 从 Cookie 中读取 `token`，用 HMAC 验证 JWT
- 无效/缺失时：返回 401 JSON `{"error": "unauthorized"}`
- 不做重定向，由前端决定行为

### 3. 新建 `handlers/auth.go`

- `LoginHandler(authPassword, jwtSecret string)` — POST `/api/login`
  - 接收 JSON `{"password": "..."}`
  - 用 `crypto/subtle.ConstantTimeCompare` 对比密码
  - 成功：签发 7 天有效期 JWT，设置 HttpOnly + SameSite=Lax Cookie
  - 失败：返回 401
- `LogoutHandler` — POST `/api/logout`
  - 清除 Cookie，返回 200

### 4. 新建 `templates/login.html`（或在现有页面中嵌入登录组件）

- 简洁的密码输入表单
- 提交密码到 `/api/login`，成功后刷新页面

### 5. 修改 `main.go`

- 启动时读取环境变量 `AUTH_PASSWORD` 和 `JWT_SECRET`，缺失则 Fatal
- 路由分组：
  - **公开路由**：所有页面路由（GET /、/online-note 等）、`POST /api/login`、`POST /api/logout`、静态资源
  - **受保护路由**（使用 AuthRequired 中间件）：所有数据 API（/notes/*、/passwords/*、/concat、/convert 等）

### 6. 前端双模式存储逻辑

核心思路：**纯前端判断登录状态，无需发请求**。登录成功时在 localStorage 存标记，页面加载时检查该标记决定存储方式。

#### 登录状态管理（纯前端，无网络请求）：
```javascript
// 登录成功后（在登录回调中）
localStorage.setItem('authenticated', 'true');

// 页面加载时检查（同步，无请求）
const isAuthenticated = localStorage.getItem('authenticated') === 'true';

// 登出时清除
localStorage.removeItem('authenticated');

// 容错：如果标记存在但 Cookie 已过期，API 返回 401 时清除标记
function handleApiError(res) {
  if (res.status === 401) {
    localStorage.removeItem('authenticated');
    // 提示用户重新登录
  }
}
```

#### `templates/notes.html` 改造：
```javascript
const isAuthenticated = localStorage.getItem('authenticated') === 'true';

// 读取笔记
async function loadNotes() {
  if (isAuthenticated) {
    const res = await fetch('/notes');
    if (res.status === 401) { localStorage.removeItem('authenticated'); /* 提示重新登录 */ return []; }
    return await res.json();
  } else {
    return JSON.parse(localStorage.getItem('notes') || '[]');
  }
}

// 保存笔记
async function saveNote(note) {
  if (isAuthenticated) {
    await fetch('/notes', { method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify(note) });
  } else {
    const notes = JSON.parse(localStorage.getItem('notes') || '[]');
    note.id = Date.now();
    notes.push(note);
    localStorage.setItem('notes', JSON.stringify(notes));
  }
}
// 类似处理 update、delete
```

#### passwordx（密码管理）页面：
- 同样的双模式逻辑
- 已登录 → 调用 /passwords API
- 未登录 → localStorage 存储

#### 登录/登出 UI：
- 在每个页面的导航栏或角落添加登录状态显示
- 未登录：显示「登录」按钮，点击弹出密码输入框
- 已登录：显示「已登录」+ 「登出」按钮

### 7. 修改 `Dockerfile`

- 添加 `AUTH_PASSWORD` 和 `JWT_SECRET` 环境变量说明

## 文件变更清单

| 文件 | 操作 | 说明 |
|------|------|------|
| `go.mod` / `go.sum` | 修改 | 添加 jwt/v5 依赖 |
| `middleware/auth.go` | 新建 | JWT Cookie 验证中间件 |
| `handlers/auth.go` | 新建 | 登录/登出处理 |
| `main.go` | 修改 | 环境变量读取、路由分组（页面公开，API 受保护） |
| `templates/notes.html` | 修改 | 双模式存储（远程/localStorage）+ 登录 UI |
| `static/vault.html` | 修改 | 双模式存储 + 登录 UI |
| `templates/index.html` | 修改 | 添加登录入口 |
| `Dockerfile` | 修改 | ENV 变量文档 |

## 验证方式

```bash
AUTH_PASSWORD=mypass JWT_SECRET=some-random-secret-key go run .
```

1. 访问任意页面 → 正常显示，显示「未登录」状态
2. 在笔记页面添加笔记 → 数据存在 localStorage
3. 点击登录，输入正确密码 → 状态变为「已登录」
4. 添加笔记 → 数据通过 API 存到服务器
5. 直接调用 API（无 Cookie）→ 返回 401
6. 登出 → 回到本地存储模式
