# news.lssl.work 晨报站点部署（全部走平台，宿主机不装任何东西）

晨报内容在 GitHub（`cool-sai/daily-news`）维护；每次 push 由 GitHub webhook 通知服务器立即浅拉到 `/srv/daily-news`
（没有定时兜底）。三个容器都按平台原生方式（SCM 任务 + 应用）部署，`tlb` 网关按 `news.lssl.work` 转发并终止 HTTPS。

```
用户 ──443──> tlb (news.lssl.work, 证书 live/news.lssl.work)
               ├─ /       ──> daily-news:80       （nginx，只读挂载 /srv/daily-news）
               └─ /_hook  ──> daily-news-hook:8080（校验签名后 git fetch 到 /srv/daily-news）
cert-renew ──写证书──> letsencrypt 卷；挑战文件写 tlb-acme 卷（tlb 80 端口 /.well-known/acme-challenge/）
```

## 文件

- `projects/daily-news/`：`Dockerfile`（`nginx:1.27-alpine`，`nginx.conf` 打进镜像并 `nginx -t`）、`nginx.conf`、`compose.yml`。
- `projects/daily-news-hook/`：Python 标准库实现，`app.py`、`test_app.py`（构建时运行）、`Dockerfile`、`compose.yml`。
  - `POST /github`：校验 `X-Hub-Signature-256`（`hmac.compare_digest`；失败 401），body ≤1MB（超出 413）；`ping` 回 200；
    只有 `cool-sai/daily-news` 的 `refs/heads/main` push 触发拉取（202），其他回 202 不拉。拉取后台串行，期间的推送合并成一次补拉。
    tlb 去掉路由前缀 `/_hook`，所以上游收到 `/github`；`/_hook/github` 也接受。`GET /health` 返回 ok。
  - 启动时 `/srv/daily-news` 为空或不是 git 仓库 → 自动首次拉取。拉取是**原地** `git init/fetch/reset`，不换目录，
    daily-news 容器的绑定挂载能立刻看到新内容。失败保留旧内容。
  - `DAILY_NEWS_HOOK_SECRET` 为空时拒绝启动。
- `projects/cert-renew/`：基于 `certbot/certbot`，`renew.py`、`test_renew.py`、`Dockerfile`、`compose.yml`。
  - 启动时签发 `CERT_DOMAINS` 里缺失的证书（webroot `/acme`），之后每 12 小时补签缺失的 + 对 webroot 证书 `certbot renew`；
    manual dns-01 签的（现在的 lssl.work、img.lssl.work）不动。签发失败不退出，下一轮再试（避免重启刷 Let's Encrypt 频率限制）。
  - 证书有变化（certbot deploy-hook）就通过挂载的 `docker.sock` 执行 `docker exec golang-tlb-1 nginx -t && nginx -s reload`。
    选它的原因：续期是原地换文件，nginx 只要 reload 就生效，不必改 TLB 配置；平台容器本来就挂了 docker.sock，是现成做法；
    「到平台重新发布」需要人点，自动续期做不到。**新签**的证书第一次仍需在平台重新发布 TLB，让站点配置指向 `live/<域名>/`。
- `platform/server/tlb.go`：每个站点优先用 `/etc/letsencrypt/live/<站点第一个 host>/` 的证书，没有则回落默认证书。
- 平台部署时把 `projects/<服务>/compose.yml` 去掉 `build`、固定 `image`，连同顶层 `volumes` 写成 `deploy/runtime/<服务>.yml`。
  cert-renew 里的 `letsencrypt`、`tlb-acme` 与主 `docker-compose.yml` 同名，在同一 compose 项目里即 `golang_letsencrypt`、`golang_tlb-acme`。

## 部署步骤

平台根目录在宿主机上是 `/opt/byte-basic`（平台容器里叫 `/work`），compose 会自动读取其中的 `.env`。

1. **`.env`（一次性，唯一要在服务器上手动做的事）**：在 `/opt/byte-basic/.env` 追加
   ```
   DAILY_NEWS_HOOK_SECRET=<openssl rand -hex 32 生成>
   CERT_DOMAINS=news.lssl.work
   # CERT_EMAIL=you@example.com   可选，不填就不带邮箱注册
   ```
   该文件已是 600 权限，不要提交到仓库。

2. **平台 SCM → 新建任务**（Git 都填 golang 仓库的 Gitee 地址，label 选 `image`）：
   | 名称 | 脚本 |
   |---|---|
   | `daily-news` | `projects/daily-news/Dockerfile` |
   | `daily-news-hook` | `projects/daily-news-hook/Dockerfile` |
   | `cert-renew` | `projects/cert-renew/Dockerfile` |

3. **平台 部署 → 新建应用**：名称、关联 SCM、Compose 三项都填同一个名字（`daily-news` / `daily-news-hook` / `cert-renew`），
   分别点部署、分支 `main`。已经建过的两个改完 `.env` 后重新部署一次即可。
   - daily-news-hook 日志应出现 `listening on :8080` 和 `updated none -> xxxxxxx`（自动首次拉取）。
   - cert-renew 日志应出现 `issue news.lssl.work: ok` 和 `reloaded golang-tlb-1`。

4. **平台 TLB**：站点 `news`（host `news.lssl.work`）应有两条路由：`/` → `daily-news:80`，`/_hook` → `daily-news-hook:8080`。
   证书签好后点「发布」，站点配置会改用 `live/news.lssl.work/`。

5. **验证**（任意机器）：
   ```bash
   curl -sI https://news.lssl.work/ | head -3                         # 200
   echo | openssl s_client -connect news.lssl.work:443 -servername news.lssl.work 2>/dev/null | openssl x509 -noout -subject -enddate
   curl -s https://news.lssl.work/_hook/health                        # ok
   curl -s -o /dev/null -w '%{http_code}\n' -X POST https://news.lssl.work/_hook/github   # 401
   curl -sI https://news.lssl.work/.git/config | head -1              # 404
   ```

6. **GitHub webhook**：`cool-sai/daily-news` → Settings → Webhooks：Payload URL `https://news.lssl.work/_hook/github`，
   Content type `application/json`，Secret 同 `.env`，SSL verification 开，只选 push，Active。
   Recent Deliveries 里 ping 应 200；推送或 Redeliver 应 202 `pull scheduled`。

7. 都正常后去掉 GitHub Pages 的 `news.lssl.work` 自定义域名（并删除仓库里的 CNAME 文件）。

更新代码：推 Gitee 后在平台对应应用点部署。更换 webhook 密钥：改 `.env` → 重新部署 daily-news-hook → 改 GitHub Secret。

## 现有 lssl.work / img.lssl.work 证书转为自动续期

它们是 manual dns-01 签的，cert-renew 不会续期它们（lssl.work 12/1 到期，img.lssl.work 2027-01-07 到期）。
确认域名 A 记录指向本机后，各强制重签一次转为 webroot，再把它们加入 `CERT_DOMAINS` 并重新部署 cert-renew：

```bash
docker exec golang-cert-renew-1 certbot certonly --webroot -w /acme -n --agree-tos --register-unsafely-without-email \
  --force-renewal --cert-name lssl.work -d lssl.work -d www.lssl.work
docker exec golang-tlb-1 nginx -s reload
# .env: CERT_DOMAINS=news.lssl.work;lssl.work,www.lssl.work;img.lssl.work
```

`--cert-name` 与原证书相同会原地替换，路径不变；失败时原证书不变。Let's Encrypt 同一组域名每周限 5 次，别反复强制重签。

## 回滚

- 停自动更新：GitHub 里取消 webhook 的 Active 或删除。
- 撤站点：平台 TLB 删除 `news` 站点 → 发布；平台部署里删除应用 `daily-news-hook`、`daily-news`（平台会 stop/rm 容器并删 runtime yml）；
  DNS 改回 GitHub Pages 即可恢复旧站。
- 停证书续期：平台里删除应用 `cert-renew`（已签的证书留在卷里不受影响）。
- tlb 配置：发布时 `nginx -t` 失败不会生效；必要时再发布一次旧配置。

## 风险 / 注意

- 没有定时兜底：webhook 投递失败或服务器访问 GitHub 失败时停在旧内容，可 Redeliver 或重新部署 daily-news-hook（不会自动重拉已有仓库，需 Redeliver）。
- cert-renew 挂载了 `docker.sock`（等同宿主机 root 权限），代码只用它执行 `docker exec golang-tlb-1 nginx ...`。
- 磁盘：certbot 镜像约 120MB，hook 约 135MB，nginx 约 50MB；服务器剩余约 8G。
