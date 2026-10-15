# news.lssl.work 晨报站点部署

晨报内容在 GitHub（`cool-sai/daily-news`）维护；每次 push 由 GitHub webhook 通知服务器立即浅拉到 `/srv/daily-news`
（没有定时兜底）。两个容器都按平台原生方式（SCM 任务 + 应用）部署，`tlb` 网关按 `news.lssl.work` 转发并终止 HTTPS。

```
用户 ──443──> tlb (news.lssl.work, 证书 live/news.lssl.work)
               ├─ /       ──> daily-news:80       （nginx，只读挂载 /srv/daily-news）
               └─ /_hook  ──> daily-news-hook:8080（校验签名后执行 daily-news-pull.sh，读写 /srv）
```

## 文件

- `projects/daily-news/`：`Dockerfile`（`nginx:1.27-alpine`，把 `nginx.conf` 打进镜像并 `nginx -t`）、`nginx.conf`、`compose.yml`。
- `projects/daily-news-hook/`：Python 标准库实现（`ThreadingHTTPServer` + `hmac`），`app.py`、`test_app.py`（unittest，镜像构建时运行）、`Dockerfile`（`python:3.12-alpine`）、`compose.yml`。
  本地测试：`cd projects/daily-news-hook && python3 -m unittest -v`。
  - `POST /github`：校验 `X-Hub-Signature-256`（HMAC-SHA256，`hmac.compare_digest` 常量时间比较；失败 401），body ≤1MB（超出 413）；
    `ping` 回 200；只有 `cool-sai/daily-news` 的 `refs/heads/main` push 触发拉取（202），其他事件/仓库/分支回 202 不拉。
    拉取异步串行，运行中再来的推送合并成一次补拉。tlb 会去掉路由前缀 `/_hook`，所以上游收到 `/github`；`/_hook/github` 也接受。
  - `GET /health`（或 `/_hook/health`）返回 `ok`。`DAILY_NEWS_HOOK_SECRET` 为空时拒绝启动。
- 平台部署时会把 `projects/<服务>/compose.yml` 去掉 `build`、固定 `image`，写成 `deploy/runtime/<服务>.yml` 再 `compose up`，不用手工复制。
- `platform/server/tlb.go`：每个站点优先用 `/etc/letsencrypt/live/<站点第一个 host>/` 的证书，没有则回落默认证书。
- `scripts/daily-news-pull.sh`：浅拉内容（hook 镜像内置一份；宿主机上手动用于首次 clone/补拉），flock、重试 3 次，失败保留旧内容。
- `scripts/cert.sh` + `deploy/systemd/cert-renew.{service,timer}`：`certbot/certbot` 容器 http-01 webroot 签发/续期，续期后 reload tlb。

## 部署顺序

1. **上线代码**：提交并推到 Gitee，按平时方式更新 platform（含 tlb.go 改动），同时把仓库同步到服务器 `/work`
   （`scripts/` 和 `deploy/systemd/` 要在宿主机上用）。

2. **一次性准备（服务器 root）**：
   ```bash
   cd /work
   echo "DAILY_NEWS_HOOK_SECRET=$(openssl rand -hex 32)" >> /work/.env && chmod 600 /work/.env
   grep DAILY_NEWS_HOOK_SECRET /work/.env        # 记下，GitHub 要填
   chmod +x scripts/daily-news-pull.sh scripts/cert.sh
   scripts/daily-news-pull.sh                    # 首次 clone 到 /srv/daily-news，GitHub 不通可多试几次
   ls /srv/daily-news/index.html
   ```
   `.env` 在 `/work` 下，平台执行 compose 时会自动读取；不要提交到仓库。

3. **平台 SCM → 新建任务**（两个，Git 都填 golang 仓库的 Gitee 地址，label 选 `image`）：
   - 名称 `daily-news`，脚本 `projects/daily-news/Dockerfile`
   - 名称 `daily-news-hook`，脚本 `projects/daily-news-hook/Dockerfile`

4. **平台 部署 → 新建应用**（两个）：
   - 名称 `daily-news`，关联 SCM `daily-news`，Compose `daily-news`
   - 名称 `daily-news-hook`，关联 SCM `daily-news-hook`，Compose `daily-news-hook`

   分别点部署，分支选 `main`，日志最后应是 `docker compose up ... daily-news(-hook)` 且状态 ok。
   验证：`docker exec golang-tlb-1 wget -qO- http://daily-news/ | head -3`、
   `docker exec golang-tlb-1 wget -qO- http://daily-news-hook:8080/health`（ok）、`docker logs golang-daily-news-hook-1`（listening on :8080）。

5. **改 DNS**：DNSPod 把 `news` 从 CNAME（cool-sai.github.io）改为 **A 记录 101.96.231.196**，等 `dig +short news.lssl.work` 返回该 IP。
   （此时 HTTPS 会落到默认站点、证书不匹配，属正常过渡，尽快做下一步。）

6. **签证书**（http-01，走 tlb 80 端口默认 server 的 `/.well-known/acme-challenge/`）：
   ```bash
   /work/scripts/cert.sh issue news.lssl.work 你的邮箱
   ```

7. **平台 TLB**：新建站点 `news`，host `news.lssl.work`；新建两条路由：
   path `/` → target `daily-news:80`；path `/_hook` → target `daily-news-hook:8080`；点「发布」。

8. **验证**：
   ```bash
   curl -sI https://news.lssl.work/ | head -5                    # 200，Cache-Control: no-cache
   echo | openssl s_client -connect 101.96.231.196:443 -servername news.lssl.work 2>/dev/null | openssl x509 -noout -subject -enddate
   curl -sI https://news.lssl.work/data/$(date +%F).md | head -3 # 当天晨报 200
   curl -sI https://news.lssl.work/.git/config | head -1         # 404
   curl -s https://news.lssl.work/_hook/health                   # ok
   curl -s -o /dev/null -w '%{http_code}\n' -X POST https://news.lssl.work/_hook/github  # 401
   curl -sI https://lssl.work/ | head -1                         # 旧站不受影响
   ```

9. **GitHub webhook**：`cool-sai/daily-news` → Settings → Webhooks → Add webhook：
   Payload URL `https://news.lssl.work/_hook/github`，Content type `application/json`，Secret 填第 2 步的值，
   SSL verification Enable，Just the push event，Active。保存后 Recent Deliveries 里 `ping` 应为 200；
   之后推一次晨报或对某条 push 点 Redeliver，应 202 `pull scheduled`，`docker logs golang-daily-news-hook-1` 能看到 `pull done`。

10. **证书续期定时器**：
    ```bash
    cp /work/deploy/systemd/cert-renew.* /etc/systemd/system/
    systemctl daemon-reload && systemctl enable --now cert-renew.timer
    /work/scripts/cert.sh renew   # 手动跑一次看日志，未到期会显示 not due
    ```

11. 都正常后，去 GitHub Pages 设置里去掉 `news.lssl.work` 自定义域名（仓库里的 CNAME 文件一并删除）。

更新：改 `projects/daily-news*` 后推 Gitee，平台里对应应用点部署即可。
更换密钥：改 `/work/.env`，平台里重新部署 `daily-news-hook`，再改 GitHub 里的 Secret。

## 回滚

- 停自动更新：GitHub 里把 webhook 取消 Active 或删除；之后需要时手动跑 `/work/scripts/daily-news-pull.sh`。
- 撤站点：平台 TLB 删除 `news` 站点 → 发布；平台部署里删除应用 `daily-news-hook`、`daily-news`
  （平台会 stop/rm 容器并删掉 `deploy/runtime/<服务>.yml`）；DNS 把 `news` 改回 CNAME `cool-sai.github.io`
  （GitHub Pages 自定义域名还在的话立即恢复）。
- 停续期：`systemctl disable --now cert-renew.timer`。
- 撤平台代码：回滚 platform 镜像到上一版本。旧代码对所有站点使用默认证书，若仍有 news 站点会报证书不匹配，所以先删站点再回滚。
- tlb 配置：发布时 `nginx -t` 失败不会生效；必要时用平台再发布一次旧配置。

## 现有 lssl.work / img.lssl.work 证书转为自动续期

它们是 `certbot --manual` dns-01 签的，`cert.sh renew` 只处理 webroot 证书，会**跳过**它们。
lssl.work 12/1 到期，img.lssl.work 2027-01-07 到期。确认两个域名都 A 记录指向本机后，可各重签一次转为 webroot：

```bash
CERTBOT_EXTRA=--force-renewal /work/scripts/cert.sh issue lssl.work 你的邮箱 www.lssl.work
CERTBOT_EXTRA=--force-renewal /work/scripts/cert.sh issue img.lssl.work 你的邮箱
/work/scripts/cert.sh list    # 确认域名和到期时间
```

`--cert-name` 与原证书相同，会原地替换 `live/<名字>/`，路径不变，tlb 配置不用改。
失败时原证书保持不变（certbot 只有成功才切换）。Let's Encrypt 同一组域名每周限 5 次，别反复强制重签。

## 风险 / 注意

- 磁盘剩余约 8G：两个镜像共约 150MB，hook 基于 python:3.12-alpine（约 135MB），certbot 镜像约 100MB。
- hook 容器以 root 运行并读写挂载整个 `/srv`；手动 pull 与 webhook pull 不共用锁，手动拉时避开推送。
- 没有定时兜底：webhook 投递失败或服务器访问 GitHub 失败时站点停在旧内容，直到下一次 push；可在 GitHub Recent Deliveries 里 Redeliver，或手动跑 `/work/scripts/daily-news-pull.sh`。日志看 `docker logs golang-daily-news-hook-1`（或平台运行时页面）。
- `cert.sh` 默认容器名 `golang-tlb-1`、卷名 `golang_letsencrypt` / `golang_tlb-acme`，不同可用
  `TLB_CONTAINER` / `LE_VOL` / `ACME_VOL` 覆盖。
