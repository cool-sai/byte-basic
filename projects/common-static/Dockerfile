FROM golang:1.25-alpine AS build
RUN sed -i 's#dl-cdn.alpinelinux.org#mirrors.aliyun.com#g' /etc/apk/repositories \
 && apk add --no-cache ca-certificates
WORKDIR /src
ENV GOPROXY=https://goproxy.cn,direct
COPY projects/common-static/ ./
RUN CGO_ENABLED=0 go test ./...
RUN CGO_ENABLED=0 go build -o /common-static .

FROM alpine:3.20
RUN sed -i 's#dl-cdn.alpinelinux.org#mirrors.aliyun.com#g' /etc/apk/repositories \
 && apk add --no-cache ca-certificates tzdata
COPY --from=build /common-static /common-static
ENV TZ=Asia/Shanghai
ENTRYPOINT ["/common-static"]
