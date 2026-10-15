"""python3 -m unittest -v（在本目录下运行）"""
import hashlib
import hmac
import http.client
import threading
import time
import unittest
from http.server import ThreadingHTTPServer

import app

SECRET = b"s3cret"
REPO, REF = "cool-sai/daily-news", "refs/heads/main"


def sign(body: bytes, secret: bytes = SECRET) -> str:
    return "sha256=" + hmac.new(secret, body, hashlib.sha256).hexdigest()


def push(repo=REPO, ref=REF) -> bytes:
    return ('{"ref":"%s","repository":{"full_name":"%s"}}' % (ref, repo)).encode()


class TestSignature(unittest.TestCase):
    def test_cases(self):
        body = b'{"a":1}'
        good = sign(body)
        cases = [
            ("ok", SECRET, good, True),
            ("wrong secret", b"other", good, False),
            ("empty secret", b"", good, False),
            ("no prefix", SECRET, good[len("sha256="):], False),
            ("sha1", SECRET, "sha1=" + good[len("sha256="):], False),
            ("bad hex", SECRET, "sha256=zz", False),
            ("short", SECRET, good[:20], False),
            ("empty", SECRET, "", False),
        ]
        for name, secret, header, want in cases:
            self.assertEqual(app.verify_signature(secret, body, header), want, name)
        self.assertFalse(app.verify_signature(SECRET, b'{"a":2}', good), "tampered body")


class TestDecide(unittest.TestCase):
    def test_cases(self):
        cases = [
            ("ping", "ping", b"{}", 200, False),
            ("push main", "push", push(), 202, True),
            ("repo case-insensitive", "push", push("Cool-Sai/Daily-News"), 202, True),
            ("other branch", "push", push(ref="refs/heads/dev"), 202, False),
            ("tag", "push", push(ref="refs/tags/main"), 202, False),
            ("other repo", "push", push("cool-sai/news-pipeline"), 202, False),
            ("issues event", "issues", push(), 202, False),
            ("bad json", "push", b"{", 400, False),
            ("json not object", "push", b"[]", 400, False),
        ]
        for name, event, body, code, pull in cases:
            got = app.decide(REPO, REF, event, body)
            self.assertEqual((got[0], got[2]), (code, pull), name)


class TestPuller(unittest.TestCase):
    def test_serialized_and_coalesced(self):
        release, finished = threading.Event(), threading.Semaphore(0)
        state = {"active": 0, "max": 0, "runs": 0}
        lock = threading.Lock()

        def run():
            with lock:
                state["active"] += 1
                state["max"] = max(state["max"], state["active"])
            release.wait()
            with lock:
                state["active"] -= 1
                state["runs"] += 1
            finished.release()

        p = app.Puller(run)
        for _ in range(5):  # 第 1 次开始跑，后 4 次合并成 1 次补拉
            p.trigger()
        release.set()
        for _ in range(2):
            self.assertTrue(finished.acquire(timeout=2))
        time.sleep(0.05)
        self.assertEqual((state["runs"], state["max"]), (2, 1))


class TestHTTP(unittest.TestCase):
    def setUp(self):
        self.pulled = threading.Event()
        handler = app.make_handler(SECRET, REPO, REF, app.Puller(self.pulled.set))
        self.srv = ThreadingHTTPServer(("127.0.0.1", 0), handler)
        threading.Thread(target=self.srv.serve_forever, daemon=True).start()

    def tearDown(self):
        self.srv.shutdown()
        self.srv.server_close()

    def req(self, method, path, body=b"", headers=None):
        c = http.client.HTTPConnection("127.0.0.1", self.srv.server_port, timeout=5)
        c.request(method, path, body=body, headers=headers or {})
        code = c.getresponse().status
        c.close()
        return code

    def hook(self, path, body, event="push", sig=None):
        return self.req("POST", path, body, {"X-GitHub-Event": event,
                                             "X-Hub-Signature-256": sig if sig is not None else sign(body)})

    def test_routes(self):
        self.assertEqual(self.req("GET", "/health"), 200)
        self.assertEqual(self.req("GET", "/_hook/health"), 200)
        self.assertEqual(self.req("GET", "/github"), 405)
        self.assertEqual(self.req("GET", "/nope"), 404)
        self.assertEqual(self.hook("/github", push(), sig="sha256=00"), 401)
        # 只声明超大 Content-Length，服务端不读 body 直接 413（真发 1MB 会因服务端断开而 BrokenPipe）
        self.assertEqual(self.req("POST", "/github", b"", {"Content-Length": str(app.MAX_BODY + 1)}), 413)
        self.assertEqual(self.hook("/github", b"{}", event="ping"), 200)
        self.assertEqual(self.hook("/github", push(ref="refs/heads/dev")), 202)
        self.assertFalse(self.pulled.is_set(), "other branch must not pull")
        self.assertEqual(self.hook("/_hook/github", push()), 202)
        self.assertTrue(self.pulled.wait(2), "pull not triggered")


class TestGitPull(unittest.TestCase):
    """用本地临时仓库模拟 GitHub，验证空目录首次拉取与原地更新（目录 inode 不变）。"""

    def test_clone_into_empty_dir_then_update(self):
        import os, subprocess, tempfile
        tmp = tempfile.mkdtemp()
        src, dst = os.path.join(tmp, "src"), os.path.join(tmp, "dst")
        g = lambda *a: subprocess.run(["git", "-C", src, *a], check=True, capture_output=True)
        os.makedirs(src)
        g("init", "-q", "-b", "main")
        g("config", "user.email", "t@t"); g("config", "user.name", "t")
        open(os.path.join(src, "index.html"), "w").write("v1")
        g("add", "."); g("commit", "-qm", "v1")
        os.makedirs(dst)  # 已存在的空目录（模拟宿主机挂载点）
        ino = os.stat(dst).st_ino
        url = "file://" + src
        self.assertTrue(app.git_pull(dst, url, "main"))
        self.assertEqual(open(os.path.join(dst, "index.html")).read(), "v1")
        open(os.path.join(src, "index.html"), "w").write("v2")
        g("commit", "-qam", "v2")
        self.assertTrue(app.git_pull(dst, url, "main"))
        self.assertEqual(open(os.path.join(dst, "index.html")).read(), "v2")
        self.assertEqual(os.stat(dst).st_ino, ino, "must update in place")
        self.assertFalse(app.git_pull(dst, "file:///nonexistent", "main", tries=1))
        self.assertEqual(open(os.path.join(dst, "index.html")).read(), "v2", "keep old content on failure")


if __name__ == "__main__":
    unittest.main()
