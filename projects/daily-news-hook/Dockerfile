# 平台 SCM（label=image）以仓库根目录为上下文构建：脚本填 projects/daily-news-hook/Dockerfile
FROM python:3.12-alpine
# git/bash/flock(util-linux)/timeout(coreutils) 给 daily-news-pull.sh；safe.directory 防止挂载目录属主不同时 git 拒绝操作
RUN sed -i 's#dl-cdn.alpinelinux.org#mirrors.aliyun.com#g' /etc/apk/repositories \
 && apk add --no-cache git bash util-linux coreutils ca-certificates tzdata \
 && git config --system --add safe.directory '*'
WORKDIR /app
COPY projects/daily-news-hook/app.py projects/daily-news-hook/test_app.py ./
RUN python -m unittest -v 2>&1 && rm -rf test_app.py __pycache__
COPY scripts/daily-news-pull.sh /usr/local/bin/daily-news-pull.sh
RUN chmod 755 /usr/local/bin/daily-news-pull.sh
ENV TZ=Asia/Shanghai PYTHONUNBUFFERED=1
EXPOSE 8080
CMD ["python", "/app/app.py"]
