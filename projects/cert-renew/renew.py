"""cert-renew：用 certbot http-01 webroot 签发/续期证书，证书变化后让 tlb 重新加载。

环境变量：
  CERT_DOMAINS   要管理的证书，多张用 ';' 分隔，每张内的多个域名用 ',' 分隔，
                 第一个域名就是证书名（/etc/letsencrypt/live/<第一个域名>/）。
                 例："news.lssl.work;lssl.work,www.lssl.work"
  CERT_EMAIL     可选，Let's Encrypt 账号邮箱；不填则不带邮箱注册。
  CERTBOT_EXTRA  可选，额外参数，例如 "--staging" 或 "--dry-run"（测试用）。
  TLB_CONTAINER  证书变化后要 reload 的 nginx 容器，默认 golang-tlb-1。
  RENEW_INTERVAL 检查间隔秒数，默认 43200（一天两次）。

流程：启动时签发缺失的证书 → 之后每隔 RENEW_INTERVAL 再补签缺失的、并对 webroot 证书执行 renew。
manual（dns-01）签的旧证书不在 renew 范围内，不会被动到。
"""
import glob
import os
import re
import shlex
import subprocess
import time

LE = "/etc/letsencrypt"
WEBROOT = "/acme"  # tlb 在 80 端口把 /.well-known/acme-challenge/ 映射到这个目录
FLAG = "/tmp/cert-changed"  # certbot --deploy-hook 成功签发/续期后会 touch 它


def log(msg: str):
    print(time.strftime("%F %T"), msg, flush=True)


def parse_domains(raw: str) -> list[list[str]]:
    """"a.com,www.a.com; b.com" -> [["a.com", "www.a.com"], ["b.com"]]"""
    certs = []
    for part in raw.split(";"):
        names = [d.strip().lower() for d in part.split(",") if d.strip()]
        if names:
            certs.append(names)
    return certs


def webroot_cert_names(le: str = LE) -> list[str]:
    """renewal/*.conf 里 authenticator = webroot 的证书名（跳过 manual dns-01 签的）。"""
    out = []
    for conf in sorted(glob.glob(os.path.join(le, "renewal", "*.conf"))):
        with open(conf) as f:
            if re.search(r"^authenticator\s*=\s*webroot\s*$", f.read(), re.M):
                out.append(os.path.basename(conf)[: -len(".conf")])
    return out


def certbot(*args: str) -> bool:
    cmd = ["certbot", *args, "--non-interactive", "--deploy-hook", f"touch {FLAG}",
           *shlex.split(os.environ.get("CERTBOT_EXTRA", ""))]
    log("$ " + " ".join(cmd))
    return subprocess.run(cmd).returncode == 0


def issue_missing(certs: list[list[str]], email: str):
    for names in certs:
        name = names[0]
        if os.path.exists(os.path.join(LE, "live", name, "fullchain.pem")):
            continue
        account = ["-m", email] if email else ["--register-unsafely-without-email"]
        domains = [x for d in names for x in ("-d", d)]
        ok = certbot("certonly", "--webroot", "-w", WEBROOT, "--agree-tos", *account,
                     "--cert-name", name, *domains)
        log(f"issue {name}: {'ok' if ok else 'FAILED, retry next round'}")


def renew_all():
    for name in webroot_cert_names():
        certbot("renew", "--cert-name", name)  # 未到期时 certbot 自己会跳过


def reload_tlb(container: str):
    """证书文件换了，nginx 要 reload 才会用新证书。通过挂载的 docker.sock 在 tlb 里执行。"""
    for args in (["nginx", "-t"], ["nginx", "-s", "reload"]):
        r = subprocess.run(["docker", "exec", container, *args])
        if r.returncode != 0:
            log(f"reload {container} failed at {' '.join(args)}")
            return
    log(f"reloaded {container}")


def main():
    certs = parse_domains(os.environ.get("CERT_DOMAINS", ""))
    email = os.environ.get("CERT_EMAIL", "").strip()
    tlb = os.environ.get("TLB_CONTAINER", "golang-tlb-1")
    interval = int(os.environ.get("RENEW_INTERVAL", "43200"))
    log(f"cert-renew start: certs={certs} email={'set' if email else 'none'} interval={interval}s")
    first = True
    while True:
        issue_missing(certs, email)  # 失败不退出，避免容器重启刷爆 Let's Encrypt 频率限制
        if not first:
            renew_all()
        first = False
        if os.path.exists(FLAG):
            os.remove(FLAG)
            reload_tlb(tlb)
        time.sleep(interval)


if __name__ == "__main__":
    main()
