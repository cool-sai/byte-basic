# 平台 SCM（label=image）以仓库根目录为上下文
FROM nginx:1.27-alpine
COPY projects/pages-web/nginx.conf /etc/nginx/conf.d/default.conf
# 占位，运行时用 compose 挂载 dist/home、dist/shell
RUN mkdir -p /usr/share/nginx/html/home /usr/share/nginx/html/shell \
 && nginx -t
