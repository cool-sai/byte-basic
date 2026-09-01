# 一个服务一份产物：build-arg BIN 决定拷哪个二进制。
# 先在本机交叉编译：见 scripts/compose-up.sh
FROM scratch
ARG BIN
COPY bin/${BIN} /app
ENTRYPOINT ["/app"]
