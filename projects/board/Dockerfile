FROM golang:1.25-alpine AS build
RUN sed -i 's#dl-cdn.alpinelinux.org#mirrors.aliyun.com#g' /etc/apk/repositories \
 && apk add --no-cache ca-certificates
WORKDIR /src
ENV GOPROXY=https://goproxy.cn,direct
COPY projects/board/go.mod projects/board/go.sum ./
RUN go mod download
COPY projects/board/ ./
RUN CGO_ENABLED=0 go test ./...
RUN CGO_ENABLED=0 go build -o /board .

FROM alpine:3.20
RUN sed -i 's#dl-cdn.alpinelinux.org#mirrors.aliyun.com#g' /etc/apk/repositories \
 && apk add --no-cache ca-certificates tzdata
COPY --from=build /board /board
ENV TZ=Asia/Shanghai
ENTRYPOINT ["/board"]
