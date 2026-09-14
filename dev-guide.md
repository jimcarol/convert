# 开发指南（dev-guide）

## 开发模式 Docker（热重载，不用反复 build 镜像）

生产环境的 `docker-compose.yml` 里，Go 代码是编译进镜像的，每次改代码都要 `up --build`。
开发时用 **`docker-compose.dev.yml`**：源码直接挂载进容器，容器内用 [air](https://github.com/air-verse/air)
监听文件变化并自动重编译、重启进程。

### 快速开始

```bash
# 首次：构建一次开发镜像（Dockerfile.dev，之后会被缓存）
docker compose -f docker-compose.dev.yml build

# 日常开发
docker compose -f docker-compose.dev.yml up

# 连 video 服务一起跑（默认不启动）
docker compose -f docker-compose.dev.yml --profile video up
```

### 访问入口

| 入口 | 地址 | 说明 |
| --- | --- | --- |
| 统一入口（caddy） | http://localhost:8080 | 与生产一致，按路径路由到各服务 |
| lite 直连 | http://localhost:8081 | 调试用 |
| heavy 直连 | http://localhost:8082 | 调试用 |
| tts 直连 | http://localhost:8083 | 调试用 |
| video 直连 | http://localhost:8090 | 仅 `--profile video` 时 |

### 改动什么会立刻生效

| 改动类型 | 生效方式 |
| --- | --- |
| Go 代码（`cmd/`、`handlers/`、`internal/`、`middleware/`） | air 自动重编译并重启进程 |
| HTML 模板（`templates/`） | 同上（gin 启动时加载模板，air 监听 `.html` 会自动重启） |
| Python 脚本（`scripts/`） | 每次运行时读取，立即生效 |
| Caddy 路由（`Caddyfile.example`） | 文件已挂载，改完执行 `docker compose -f docker-compose.dev.yml restart caddy` |
| Go 依赖变化（`go.mod`） | air 重编译时会自动拉取（module 缓存在命名卷里，很快） |

如果 macOS 上偶尔 air 没监听到变化（Docker Desktop 挂载卷的已知小概率问题），手动
`docker compose -f docker-compose.dev.yml restart <服务名>` 即可，同样不需要 rebuild。

### 设计说明

- **`Dockerfile.dev`**：开发专用镜像，基于 `golang:1.24.2-bookworm`，预装 air、python3 + edge-tts、
  ffmpeg、faster-whisper、中文字体。镜像约 2GB，但只在本地构建一次，**不影响生产镜像体积**
  （生产各服务仍用各自的 `Dockerfile.lite` / `Dockerfile.heavy` / `Dockerfile.tts`）。
  air 固定为 v1.62.0（v1.63+ 要求 Go >= 1.25，与 go1.24.2 不兼容）。
- **四个 Go 服务共用同一个镜像 `file-converter-dev`**：build 段只写在 lite 服务上，
  `docker compose -f docker-compose.dev.yml build` 即构建该共享镜像。不要把 build 段复制到其他
  服务——并发构建同一镜像名会触发 buildkit "image already exists" 冲突；其余服务用
  `pull_policy: never` 直接复用本地镜像。显式 `image:` 名字也避免了和生产 compose 的
  镜像名（`file-converter-lite` 等，同项目名推导）撞车。
- **Go module / 构建缓存用命名卷**（`gomodcache`、`gobuildcache`）：删掉容器重来后 air 的重编译仍是秒级。
- **`tmp/`、`video-tmp/`、`.git` 被排除在 air 监听之外**（`--build.exclude_dir`），否则服务运行中
  写临时文件会误触发重编译。
- **`FONT_PATH` 路径差异**：dev 镜像是 Debian，中文字体在
  `/usr/share/fonts/truetype/wqy/wqy-zenhei.ttc`；生产 alpine 在
  `/usr/share/fonts/wqy-zenhei/wqy-zenhei.ttc`。compose 里已分别配好。
- **video 服务**：容器内强制 `BIND=0.0.0.0`（默认 127.0.0.1 会导致宿主机端口映射访问不到）；
  并挂载宿主机 `~/.cache/faster-whisper`（只读）复用已下载的 whisper 模型，不用重新从 ModelScope 拉。

## 生产镜像构建（heavy 的 base image）

`Dockerfile.heavy` 的运行时依赖（LibreOffice + JRE + 中文字体，数百 MB）拆到了
**`Dockerfile.heavy-base`**，只构建一次、打好 tag `converter-heavy-base:v1`；
日常构建 heavy 只剩 Go 编译（有 BuildKit 缓存挂载，增量编译）+ 拷贝二进制。

```bash
# 日常构建 heavy(base 缺失时会自动先构建 base)
bash scripts/build-heavy.sh

# 手动构建/重建 base(仅当 LibreOffice 等依赖需要变更时)
docker compose --profile base build heavy-base
```

- base 依赖变更时:bump tag(`v1` → `v2`),同步改 `Dockerfile.heavy` 的 `FROM`
  和 `docker-compose.yml` 里 `heavy-base` 服务的 `image`。
- **架构固定 amd64**:服务器是 amd64,compose 里 `heavy`/`heavy-base` 都钉了
  `platform: linux/amd64`,本地(ARM)构建、运行都走模拟——慢一点,但保证
  `converter-heavy-base:v1` 这个 tag 本地永远只有一份 amd64,不会和原生 arm64
  构建互相覆盖。追求原生速度的日常开发请用 `docker-compose.dev.yml`。
- 服务器部署:先在那台机器上建一次 base(同上命令),或把 base 推到私有 registry 后
  修改 `Dockerfile.heavy` 的 `FROM` 指向 registry 地址。

## 本地裸跑（不用 Docker）

各服务需在 **repo 根目录** 启动（模板、脚本是相对路径）：

```bash
# tts（需要 JWT_SECRET；模板在 templates/）
JWT_SECRET=dev go run ./cmd/tts

# lite
AUTH_PASSWORD=dev JWT_SECRET=dev go run ./cmd/lite

# heavy
JWT_SECRET=dev FONT_PATH=<本机中文字体路径> go run ./cmd/heavy

# video（默认只监听 127.0.0.1:8090，无鉴权，仅限本机使用）
KIMI_API_KEY=sk-... go run ./cmd/video
```

### 依赖

- **ffmpeg**：`brew install ffmpeg`
- **edge-tts**：`pip3 install edge-tts`（或 `pipx install edge-tts`）
- **faster-whisper**（video 转写）：`pip3 install faster-whisper`，
  然后用 `scripts/download_whisper_model.sh small` 从 ModelScope 下载模型
  （国内无法直连 HuggingFace）
- **facefusion**（video 换脸，可选）：未安装时换脸阶段会报错，配音链路不受影响

## 注意事项

- 不要把 `docker-compose.dev.yml` 用于生产部署——它把源码整个挂进容器且无 `restart` 策略。
- video 服务无鉴权，若需要暴露到局域网/公网，先加鉴权。
- `JWT_SECRET` / `AUTH_PASSWORD` 通过环境变量传入，compose 里的 `dev-*` 默认值仅用于本地。
