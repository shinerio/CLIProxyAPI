# 安装与更新说明

本项目提供一个本地构建的一键安装或更新脚本：

```bash
./backend/scripts/install-or-update.sh
```

脚本默认使用当前仓库内的：

- 后端源码：`./backend`
- 前端源码：`./frontend`

不会主动从远端下载项目自身资源。前后端产物都来自本地源码与本地安装目录。

## 前置要求

执行前请确认本机已经具备：

- `go`
- `npm`
- 前端依赖目录 `./frontend/node_modules`

说明：

- 后端构建使用 `go build -mod=readonly`
- 前端构建直接使用本地已有依赖，不会执行 `npm ci`

## 基本用法

首次安装到指定目录：

```bash
./backend/scripts/install-or-update.sh --install-dir /opt/cliproxyapi
```

更新已有安装目录：

```bash
./backend/scripts/install-or-update.sh --install-dir /opt/cliproxyapi
```

更新后重启 systemd 服务：

```bash
./backend/scripts/install-or-update.sh \
  --install-dir /opt/cliproxyapi \
  --service-name cliproxyapi \
  --restart
```

只更新后端，不重新安装前端静态资源：

```bash
./backend/scripts/install-or-update.sh \
  --install-dir /opt/cliproxyapi \
  --no-frontend
```

如果目标目录非空，但不是现有安装目录，可强制继续：

```bash
./backend/scripts/install-or-update.sh \
  --install-dir /opt/cliproxyapi \
  --force
```

## 安装结果

脚本会在目标目录生成或更新：

- `cli-proxy-api`：后端二进制
- `config.yaml`：首次安装时从 `backend/config.example.yaml` 复制
- `static/`：前端静态资源
- `version.txt`：构建版本信息
- `backups/`：更新前备份

说明：

- 首次安装时才会创建 `config.yaml`
- 后续更新不会覆盖已有 `config.yaml`
- 前端资源安装到 `<install-dir>/static`

## 查看帮助

```bash
./backend/scripts/install-or-update.sh --help
```
