# 本机开发环境（Mac）— home 落地页

不依赖本机 Node / Go；**只用 Docker Desktop + compose**。

## 前置

```bash
export PATH="/Applications/Docker.app/Contents/Resources/bin:$HOME/.docker/bin:$PATH"
open -a Docker   # 等 daemon 就绪
docker info
```

Dockerfile 已用 `docker.m.daocloud.io/library/...` 拉基础镜像。若仍超时，可给 Docker Desktop 配系统代理（本机 Clash `7897`）。

## 起 home（推荐）

在仓库根：

```bash
cd ~/Desktop/projects/byte-basic
docker compose -f home/compose.yml up -d --build
```

| 入口 | URL |
|------|-----|
| 炫酷首页 | http://127.0.0.1:8080/ |
| 项目页 | http://127.0.0.1:8080/projects |
| API | http://127.0.0.1:8082/api/sites |
| API（经 nginx） | http://127.0.0.1:8080/api/sites |

停掉：

```bash
docker compose -f home/compose.yml down
```

## 说明

- `home-web`：容器内 `npm ci && build`，nginx 托管 SPA，`/api` 反代到 `home-api`
- `home-api`：容器内 `go build ./home/server`
- 上线仍走部署平台 SCM（`scripts/scm/home-web.sh` / `home-api.sh`），本 compose 只做本机验证
