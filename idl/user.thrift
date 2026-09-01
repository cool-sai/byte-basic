// 契约。生成代码、泛化调用、Python/JS 客户端都只认这份文件。
// 数字是 field id：线上协议认的是 id，不是字段名。
// 没有 agw.uri 的方法只走 RPC，不对外开放 HTTP。

namespace go user

struct GetUserReq {
    1: i64 id
}

struct GetUserResp {
    1: i64 id
    2: string name
}

struct PingReq {
}

struct PingResp {
    1: string msg
}

service UserService {
    GetUserResp GetUser(1: GetUserReq req)
    PingResp Ping(1: PingReq req)
}
