# 本地模拟基建（以后按步做）

当前做到第 8 步 + 基建控制台：SCM 编译出版本、BAM 托管 IDL、AGW 按注解开通 HTTP、部署页选版本发布。第 9 步消息队列还没做。

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

## 第 6 步怎么跑

```bash
./scripts/compose-up.sh -d
go run ./example/order_client
open http://127.0.0.1:16686
```

Jaeger → Service `order` → Find Traces。一条 GetOrder 下面挂一条 GetUser，同一个 `trace` id。

RPC 帧在 method 和 body 之间加了 2 字节 header 长度：有上游就把 `traceID(16) + parentSpanID(8)` 塞进去。order 处理完调 user 时，把当前 span 当 parent 写进帧。Jaeger 挂了只是不上报，RPC 照跑。

日志 JSON 里也有 `trace` 字段，Loki 可以 `{psm="order"} |= "同一串 hex"`。

## 第 7 步怎么跑

```bash
./scripts/compose-up.sh -d
go run ./example/order_client          # userName=alice
open http://127.0.0.1:2381             # 改 user/name_suffix
# 或 ./scripts/config-put.sh user/name_suffix '!!!'
go run ./example/order_client          # userName=alice!!!
```

没配 `CONFIG` 时只用环境变量 `NAME_SUFFIX`（启动时读一次）。配了就 watch etcd 的 `user/name_suffix`，两个 user 实例一起变，不用滚动重启。etcd 挂了继续用内存里上一次的值。

## 第 8 步怎么跑（当前）

```bash
./scripts/compose-up.sh -d
curl -s -d '{"id":1001}' -H 'Content-Type: application/json' http://127.0.0.1:18080/order/get
curl -s -d '{"userId":1,"amount":99}' -H 'Content-Type: application/json' http://127.0.0.1:18080/order/create
```

HTTP 路径来自 `idl/order.thrift` 的 `agw.uri`，网关不手写每个接口。内部仍是 RPC。没写 `agw.uri` 的方法（如 user）只走 RPC。`order_client` 还能直接打 8889。

一服务一镜像（`minikitex-user` / `minikitex-order` / `minikitex-gateway`），IDL 挂载进网关，不是编进二进制：

```bash
# 改 agw.uri：发布 IDL，不用编、不发 order
docker compose restart gateway

# 改 gateway 代码：只发网关
./scripts/compose-up.sh -d gateway

# 改 order 代码：只发 order
./scripts/compose-up.sh -d order
```

## 基建控制台（SCM / BAM / AGW / 部署）

本机没有独立 MySQL，用 compose 里的 `mysql`（root / minikitex，库 `minikitex`）。控制台「MySQL」页看表结构；或 http://127.0.0.1:18081 Adminer。更好看用桌面客户端连 `127.0.0.1:3306`（DBeaver / Sequel Ace / TablePlus）。

```bash
docker compose up -d mysql
go run ./platform/server          # :8081
# 另一个终端
cd platform/web && npm install && npm run dev
open http://127.0.0.1:5173
```

| 页 | 对应线上 | 实际做什么 |
|---|---|---|
| SCM 编译 | SCM 出制品 | `go build` linux 二进制，落到 `artifacts/<服务>/<版本>/` |
| BAM IDL | BAM 托管契约 | 编辑 `idl/*.thrift`，解析 agw.uri 和入参/出参 |
| AGW 网关 | AGW 开通 HTTP | 重启 gateway，让它重新读挂载的 IDL |
| TLB | 流量入口 | 三级域名各一份配置，nginx 按 Host + 路径转到服务，对外 :80 |
| 部署 | 发布平台选版本 | golang 打 scratch 镜像；node 打 nginx 静态镜像，再 `compose up` |

改 URL：BAM 改 `agw.uri` 保存 → AGW 点发布。不用重编 order。
改代码：SCM 编译 → 部署选这个版本。
