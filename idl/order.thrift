// 第二份契约。和 user.thrift 平级，各自生成各自的 Go 包。
// agw.method / agw.uri 是给网关看的：发布这份 IDL 后，HTTP 路径自动挂上，不用在网关里手写路由。

namespace go order

struct GetOrderReq {
    1: i64 id
}

struct GetOrderResp {
    1: i64 id
    2: i64 userId
    3: string status
    4: string userName
}

struct CreateOrderReq {
    1: i64 userId
    2: i64 amount
}

struct CreateOrderResp {
    1: i64 id
    2: string status
}

service OrderService {
    GetOrderResp GetOrder(1: GetOrderReq req) (agw.method = "POST", agw.uri = "/order/get_by_id")
    CreateOrderResp CreateOrder(1: CreateOrderReq req) (agw.method = "POST", agw.uri = "/order/create")
}
