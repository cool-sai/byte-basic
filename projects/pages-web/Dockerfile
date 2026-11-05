# 平台 SCM（label=image）以仓库根目录为上下文
# 多阶段：node 构建 home+shell → nginx 托管；/api/data → board /data
FROM node:20-alpine AS build
RUN sed -i 's#dl-cdn.alpinelinux.org#mirrors.aliyun.com#g' /etc/apk/repositories
WORKDIR /app
COPY projects/pages-web/package.json projects/pages-web/package-lock.json ./
RUN npm ci --registry=https://registry.npmmirror.com
COPY projects/pages-web/ ./
RUN npm run build -- home \
 && npm run build -- shell

FROM nginx:1.27-alpine
RUN sed -i 's#dl-cdn.alpinelinux.org#mirrors.aliyun.com#g' /etc/apk/repositories
COPY projects/pages-web/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist/home /usr/share/nginx/html/home
COPY --from=build /app/dist/shell /usr/share/nginx/html/shell
RUN nginx -t
