# 本机开发环境（Mac）— home 落地页

业务进程本机直接起（Node / Go），**不打业务镜像**。Docker 只用于 MySQL 等中间件（本服务当前无 DB 依赖）。

## 前置

- Go（跑 `home/server`）
- Node.js + npm（跑 `home/web` Vite）

## 起 API

仓库根：

```bash
cd ~/Desktop/projects/byte-basic
go run ./home/server
```

默认监听 `http://127.0.0.1:8082`（含 REST `/api/sites` 与 Connect）。

## 起前端

```bash
cd ~/Desktop/projects/byte-basic/home/web
npm ci   # 首次
npm run dev -- --host 127.0.0.1 --port 5175
```

| 入口 | URL |
|------|-----|
| 炫酷首页 | http://127.0.0.1:5175/ |
| 项目页 | http://127.0.0.1:5175/projects |
| API（直连） | http://127.0.0.1:8082/api/sites |

Vite 已把 `/api`（含 Connect `/api/home.v1.HomeService`）代理到 `8082`，本地不用 nginx。

## 上线

仍走部署平台 SCM：`scripts/scm/home-web.sh` / `home-api.sh`。
