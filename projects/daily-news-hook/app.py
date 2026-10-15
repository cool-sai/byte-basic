"""daily-news-hook：接收 GitHub push webhook，推送到 cool-sai/daily-news 的 main 分支后立即拉取内容；
启动时若内容目录为空，会自动做首次拉取。

只用标准库。tlb 路由 path 为 /_hook，会去掉前缀，所以上游收到 /github、/health；
直连调试时 /_hook/github、/_hook/health 也接受。
"""
import hashlib
import hmac
import json
import logging
import os
import subprocess
import sys
import threading
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

MAX_BODY = 1 << 20  # 1MB，晨报仓库的 push payload 远小于这个

log = logging.getLogger("daily-news-hook")


def verify_signature(secret: bytes, body: bytes, header: str) -> bool:
    """校验 X-Hub-Signature-256: sha256=<hex>。hmac.compare_digest 是常量时间比较。"""
    if not secret or not header.startswith("sha256="):
        return False
    expected = "sha256=" + hmac.new(secret, body, hashlib.sha256).hexdigest()
    return hmac.compare_digest(header.encode(), expected.encode())


def decide(repo: str, ref: str, event: str, body: bytes):
    """根据事件决定怎么回、要不要拉取。返回 (状态码, 说明, 是否拉取)。"""
    if event == "ping":
        return 200, "pong", False
    if event != "push":
        return 202, f"ignored event {event}", False
    try:
        p = json.loads(body)
        got_repo = p.get("repository", {}).get("full_name", "")
        got_ref = p.get("ref", "")
    except (ValueError, AttributeError):
        return 400, "bad json", False
    if got_repo.lower() != repo.lower():
        return 202, f"ignored repo {got_repo}", False
    if got_ref != ref:
        return 202, f"ignored ref {got_ref}", False
    return 202, "pull scheduled", True


class Puller:
    """保证拉取串行：正在拉时再来的请求只记一个 pending，拉完再补一次（多次推送合并）。"""

    def __init__(self, run):
        self.run = run  # 实际执行拉取的函数
        self.lock = threading.Lock()
        self.running = False
        self.pending = False

    def trigger(self):
        with self.lock:
            if self.running:
                self.pending = True
                return
            self.running = True
        threading.Thread(target=self._loop, daemon=True).start()

    def _loop(self):
        while True:
            try:
                self.run()
            except Exception:  # 拉取失败不能让线程卡住 running 状态
                log.exception("pull crashed")
            with self.lock:
                if not self.pending:
                    self.running = False
                    return
                self.pending = False


def make_handler(secret: bytes, repo: str, ref: str, puller: Puller):
    """生成请求处理类（http.server 要求传类，用闭包把配置带进去）。"""

    class Handler(BaseHTTPRequestHandler):
        def reply(self, code: int, msg: str):
            data = (msg + "\n").encode()
            self.send_response(code)
            self.send_header("Content-Type", "text/plain; charset=utf-8")
            self.send_header("Content-Length", str(len(data)))
            self.end_headers()
            self.wfile.write(data)

        def route(self):
            # 去掉查询串和可选的 /_hook 前缀
            path = self.path.split("?", 1)[0]
            return path[len("/_hook"):] if path.startswith("/_hook/") else path

        def do_GET(self):
            r = self.route()
            if r == "/health":
                self.reply(200, "ok")
            elif r == "/github":
                self.reply(405, "method not allowed")
            else:
                self.reply(404, "not found")

        def do_POST(self):
            if self.route() != "/github":
                return self.reply(404, "not found")
            length = int(self.headers.get("Content-Length") or 0)
            if length > MAX_BODY:
                self.close_connection = True  # 不读超大 body，直接断开
                return self.reply(413, "body too large")
            body = self.rfile.read(length)
            if not verify_signature(secret, body, self.headers.get("X-Hub-Signature-256", "")):
                log.warning("bad signature from %s", self.headers.get("X-Forwarded-For", ""))
                return self.reply(401, "bad signature")
            event = self.headers.get("X-GitHub-Event", "")
            code, msg, pull = decide(repo, ref, event, body)
            log.info("delivery=%s event=%s -> %d %s", self.headers.get("X-GitHub-Delivery", ""), event, code, msg)
            if pull:
                puller.trigger()  # 后台拉取，立即返回 202
            self.reply(code, msg)

        def log_message(self, *args):  # 关掉 http.server 默认的访问日志，用上面自己的
            pass

    return Handler


def git_pull(dir_: str, url: str, branch: str, tries: int = 3) -> bool:
    """把 url 的 branch 浅拉到 dir_，原地更新（不换目录，daily-news 容器的绑定挂载才能看到新内容）。

    目录为空或不是 git 仓库时会原地 git init，相当于首次 clone。失败时保留旧内容。
    """
    def git(*args, check=True):
        return subprocess.run(["git", "-C", dir_, *args], check=check, timeout=120,
                              capture_output=True, text=True).stdout.strip()

    os.makedirs(dir_, exist_ok=True)
    if not os.path.isdir(os.path.join(dir_, ".git")):
        log.info("init %s from %s", dir_, url)
        git("init", "-q")
    git("remote", "remove", "origin", check=False)
    git("remote", "add", "origin", url)
    old = git("rev-parse", "--short", "HEAD", check=False) or "none"
    for i in range(1, tries + 1):
        try:
            git("fetch", "-q", "--depth", "1", "origin", branch)
            break
        except (subprocess.CalledProcessError, subprocess.TimeoutExpired) as e:
            log.warning("fetch %d/%d failed: %s", i, tries, getattr(e, "stderr", "") or e)
            if i == tries:
                log.error("fetch failed, keep %s", old)
                return False
            time.sleep(i * 10)
    git("reset", "-q", "--hard", "FETCH_HEAD")
    git("clean", "-fdq")
    new = git("rev-parse", "--short", "HEAD")
    if new != old:
        log.info("updated %s -> %s", old, new)
        # 浅仓库回收旧对象，避免 .git 越积越大
        git("reflog", "expire", "--expire=now", "--all", check=False)
        git("gc", "-q", "--prune=now", check=False)
    else:
        log.info("already at %s", new)
    return True


def main():
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(message)s")
    secret = os.environ.get("DAILY_NEWS_HOOK_SECRET", "").strip()
    if not secret:
        log.error("DAILY_NEWS_HOOK_SECRET is empty, refuse to start")
        sys.exit(1)
    repo = os.environ.get("DAILY_NEWS_HOOK_REPO", "cool-sai/daily-news")
    branch = os.environ.get("DAILY_NEWS_BRANCH", "main")
    ref = "refs/heads/" + branch
    dir_ = os.environ.get("DAILY_NEWS_DIR", "/srv/daily-news")
    url = os.environ.get("DAILY_NEWS_REPO_URL", f"https://github.com/{repo}.git")

    puller = Puller(lambda: git_pull(dir_, url, branch))
    if not os.path.isdir(os.path.join(dir_, ".git")):
        puller.trigger()  # 空目录或不是仓库：启动时自动首次拉取（后台进行，不耽误监听）

    port = int(os.environ.get("DAILY_NEWS_HOOK_PORT", "8080"))
    server = ThreadingHTTPServer(("0.0.0.0", port), make_handler(secret.encode(), repo, ref, puller))
    log.info("daily-news-hook listening on :%d, repo=%s ref=%s dir=%s", port, repo, ref, dir_)
    server.serve_forever()


if __name__ == "__main__":
    main()
