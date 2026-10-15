# 平台 SCM（label=image）以仓库根目录为上下文构建：脚本填 projects/cert-renew/Dockerfile
FROM certbot/certbot:latest
# docker-cli：证书变化后 docker exec tlb nginx -s reload（通过挂载的 docker.sock）
RUN sed -i 's#dl-cdn.alpinelinux.org#mirrors.aliyun.com#g' /etc/apk/repositories \
 && apk add --no-cache docker-cli tzdata
WORKDIR /app
COPY projects/cert-renew/renew.py projects/cert-renew/test_renew.py ./
RUN python -m unittest -v 2>&1 && rm -rf test_renew.py __pycache__
ENV TZ=Asia/Shanghai PYTHONUNBUFFERED=1
ENTRYPOINT []
CMD ["python", "/app/renew.py"]
