# 平台 SCM（label=image）以仓库根目录为上下文构建：脚本填 projects/daily-news/Dockerfile
FROM nginx:1.27-alpine
COPY projects/daily-news/nginx.conf /etc/nginx/conf.d/default.conf
RUN nginx -t
