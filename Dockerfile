# 静态二进制，不依赖 docker hub 上的 golang/alpine（本机拉镜像常失败）。
# 先编 Linux 包：见 scripts/compose-up.sh
FROM scratch
COPY bin/user /usr/local/bin/user
COPY bin/order /usr/local/bin/order
COPY bin/order-client /usr/local/bin/order-client
COPY bin/registry /usr/local/bin/registry
