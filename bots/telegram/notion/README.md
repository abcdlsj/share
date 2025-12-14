# Telegram → Notion 随笔 Bot (Go)

目标：在 Telegram 对话里“开始/结束/新随笔”式记录，按消息顺序把文字与图片写入 Notion。

## 功能

- 命令面板：`/begin`（开始）、`/end`（结束并要标题）、`/new`（flush 并开启新随笔）、`/cancel`（丢弃上下文）
- 也支持直接发：`开始` / `结束` / `新随笔`
- 每条消息单独一段（Notion 一个 paragraph block）
- 图片：本地压缩到 **< 5MB**（可配），上传到 S3 兼容存储，再以 Notion external image 引用（保证图文顺序）

## 环境变量

支持两种方式：

1) `config.yaml`（推荐，支持 `${ENV}` 变量展开）
2) 纯环境变量

### 方式 1：config.yaml

复制 [`config.yaml.example`](config.yaml.example:1) 为 `config.yaml`，按需填写。

运行：

```bash
go run . -config config.yaml
```

`config.yaml` 内支持 `${TELEGRAM_TOKEN}` 这种写法，会在读取后做一次 `os.ExpandEnv` 展开。

Notion Database 推荐属性（可在 `config.yaml` 里改名匹配你的 DB）：

- 标题属性：默认 `Title`
- `Created`：date 属性；写入时会填当前时间（按 `notion_tz`，默认 Asia/Shanghai）
- `Visibility`：select 属性；写入时固定选择 `Private`
- `Tags`：留空（不写入）

### 方式 2：环境变量

必填：

```bash
export TELEGRAM_TOKEN=your-token

export NOTION_TOKEN=your-token
export NOTION_DATABASE_ID=your-database-id
export NOTION_TITLE_PROP=Title   # 可选，默认 Title

# 可选：如果你的 DB 属性名不同
export NOTION_CREATED_PROP=Created
export NOTION_VISIBILITY_PROP=Visibility
export NOTION_VISIBILITY_VALUE=Private
export NOTION_TZ=Asia/Shanghai

export S3_ENDPOINT=https://<your-s3-endpoint>
export S3_REGION=auto          # 兼容 R2/OSS，默认 auto
export S3_BUCKET=your-bucket
export S3_ACCESS_KEY_ID=xxx
export S3_SECRET_ACCESS_KEY=xxx

# Notion 使用 external image URL，这里必须是公网可访问的 base URL（一般是 bucket 的 public 域名）
export S3_PUBLIC_BASE_URL=https://<public-base>/<optional-prefix>

export S3_KEY_PREFIX=telegram-notes      # 可选
export S3_FORCE_PATH_STYLE=true          # 可选，某些 OSS 需要

# 可选：图片大小上限（默认 4_900_000，留余量）
export MAX_IMAGE_BYTES=100000000

# 可选：Telegram 图片下载上限（默认 30MiB）
export TG_DOWNLOAD_MAX_BYTES=31457280

# 可选：JPEG 压缩最低质量阈值（默认 45；越低越省体积，但画质更差）
export IMG_JPEG_MIN_QUALITY=45
```

## 运行

```bash
go mod tidy
go run .
```
