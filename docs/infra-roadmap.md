# 本地模拟基建（以后按步做）

当前做到第 3 步：注册中心。user 心跳上报地址，order 按服务名 `user` 拉取实例列表，不再写死 `USER_ADDR`。

Docker Compose 是 Docker 的「一次起一堆容器」工具：`docker-compose.yml` 里写镜像、端口、环境变量，`docker compose up` 一起起来。容器之间用服务名当地址（`user-1:8888`），不用写 `127.0.0.1`。它还不是 K8s，也不是服务发现。

| 步 | 做什么 | 本地用 | 线上大致是哪类东西 |
|---|---|---|---|
| 1 | 双进程 RPC | 两个 `go run` | 两个 Kitex 服务互调 |
| 2 | 容器化 | Docker Compose（user×2 + order×1） | 每服务多实例 |
| 3 | 服务发现 | 自写 registry（Consul 缩水版） | 注册中心 / Mesh 找对端 |
| 4 | 日志 | `docker logs` / 终端 stdout | 日志平台（采集、检索） |
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

## 第 3 步怎么跑（当前）

```bash
./scripts/compose-up.sh -d
curl -s 'http://127.0.0.1:8500/lookup?name=user'
go run ./example/order_client
docker compose stop user-2
# 等约 8 秒 TTL，再 lookup 只剩 user-1
curl -s 'http://127.0.0.1:8500/lookup?name=user'
```

本机 `go run` 不设 `REGISTRY`，order 仍用 `USER_ADDR`（默认 `127.0.0.1:8888`）。

这就是 Consul 的核心：名字 → 一串活着的地址。生产换 Consul/etcd，客户端协议变，模型不变。
