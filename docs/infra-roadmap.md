# 本地模拟基建（以后按步做）

当前做到第 6 步：一次 GetOrder 在 Jaeger 里能看到 order → user 两条 span。

Docker Compose 是 Docker 的「一次起一堆容器」工具：`docker-compose.yml` 里写镜像、端口、环境变量，`docker compose up` 一起起来。容器之间用服务名当地址（`user-1:8888`），不用写 `127.0.0.1`。它还不是 K8s，也不是服务发现。

| 步 | 做什么 | 本地用 | 线上大致是哪类东西 |
|---|---|---|---|
| 1 | 双进程 RPC | 两个 `go run` | 两个 Kitex 服务互调 |
| 2 | 容器化 | Docker Compose（user×2 + order×1） | 每服务多实例 |
| 3 | 服务发现 | Consul（-dev） | 注册中心 / Mesh 找对端 |
| 4 | 日志 | Loki + Grafana（仍可用 `docker compose logs`） | 日志平台（采集、检索） |
| 5 | 指标 | Prometheus + Grafana | QPS / 延迟 / 错误率 |
| 6 | 链路追踪 | Jaeger | 一次请求穿过 order→user 的时间轴 |
| 7 | 配置 | 环境变量，再往后 etcd | 动态配置中心 |
| 8 | 网关 | 一个小 HTTP 入口转发 RPC | API 网关 |
| 9 | 消息队列 | Redis list 或 Kafka | 异步解耦（下单、发事件） |
| 10 | 存储 | SQLite / Redis | MySQL / Redis / 对象存储 |
| 11 | 网格 / 发布 | 先不做 | Mesh、发布平台、权限 |

原则：

- 当前步能用手点验证，再加下一步。
- 观测三件套（日志 / 指标 / 追踪）不要第一步就上；先看进程打印。
- MQ、Mesh、发布平台不改变 Encode/Decode，和学 RPC 无关，放最后。

## 第 1 步怎么跑（本机进程）

```bash
# 终端 1
go run ./example/server

# 终端 2
go run ./example/order

# 终端 3
go run ./example/order_client
```

期望：`GetOrder -> id=1001 userId=1 status=paid userName=alice`  
order 日志里有一行 `GetOrder 1001 -> RPC user.GetUser(1)`。

## 第 2 步怎么跑（容器化）

```bash
chmod +x scripts/compose-up.sh
./scripts/compose-up.sh
# 另一个终端
go run ./example/order_client
```

本机交叉编译成 Linux 静态文件再 `FROM scratch`，不拉 docker hub 的 golang 镜像。

期望仍是 `userName=alice`。连打两次，order 日志里 `via user-1:8888` / `via user-2:8888` 轮流出现；user 容器日志带 `[user-1]` / `[user-2]`。

## 第 3 步怎么跑（Consul）

```bash
./scripts/compose-up.sh -d
curl -s 'http://127.0.0.1:8500/v1/health/service/user?passing=true'
# 浏览器：http://127.0.0.1:8500/ui
go run ./example/order_client
docker compose stop user-2
# 约 8 秒 TTL 后 passing 列表只剩 user-1
```

本机 `go run` 不设 `REGISTRY`，仍走 `USER_ADDR`。

## 第 4 步怎么跑

```bash
./scripts/compose-up.sh -d
go run ./example/order_client
docker compose logs --tail=20 order user-1
open http://127.0.0.1:3000
```

Grafana 可匿名看；收藏要登录 `admin` / `admin`。Explore → Loki：

- `{psm="user"}` — user 服务全部实例（和内部按 PSM 查一样）
- `{psm="user",instance="user-1"}` — 某一台
- `{psm="order"}` — 订单服务

JSON 里也有 `psm` / `instance` 字段。

进程仍然只写 stderr；Promtail 从 Docker 日志里抄走。Loki 挂了不影响 RPC。

## 第 5 步怎么跑

```bash
./scripts/compose-up.sh -d
# 打几次流量，否则图是空的
for i in 1 2 3 4 5; do go run ./example/order_client; done
open http://127.0.0.1:3000/d/rpc/rpc
# 或 Prometheus：http://127.0.0.1:9090
# Explore → Prometheus：rate(rpc_requests_total[30s])
```

每个 RPC 进程另开 HTTP `:9091/metrics`（Prometheus 文本格式）。Prometheus 按 `psm` / `instance` 贴标签来刮，和日志那套一样。

- `rpc_requests_total{method,status}` — 次数（QPS 用 `rate()`）
- `rpc_request_duration_seconds` — 延迟直方图（p99 用 `histogram_quantile`）

进程不主动推指标；Prometheus 来拉。Prometheus 挂了不影响 RPC。

## 第 6 步怎么跑（当前）

```bash
./scripts/compose-up.sh -d
go run ./example/order_client
open http://127.0.0.1:16686
```

Jaeger → Service `order` → Find Traces。一条 GetOrder 下面挂一条 GetUser，同一个 `trace` id。

RPC 帧在 method 和 body 之间加了 2 字节 header 长度：有上游就把 `traceID(16) + parentSpanID(8)` 塞进去。order 处理完调 user 时，把当前 span 当 parent 写进帧。Jaeger 挂了只是不上报，RPC 照跑。

日志 JSON 里也有 `trace` 字段，Loki 可以 `{psm="order"} |= "同一串 hex"`。
