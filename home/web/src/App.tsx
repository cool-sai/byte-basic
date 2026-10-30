import { useEffect, useMemo, useState, type ReactNode } from "react";
import { client } from "./api";
import type { Site } from "./gen/home/v1/home_pb";

type NavKey = "home" | "projects";

const SHOWCASE: Array<{
  name: string;
  title: string;
  tag: string;
  desc: string;
  href: string;
  accent: string;
}> = [
  {
    name: "minikitex",
    title: "minikitex",
    tag: "Platform",
    desc: "自研部署控制台：SCM / BAM / AGW / TLB / Deploy，从编译到上线一条链路。",
    href: "/minikitex/",
    accent: "#22d3ee",
  },
  {
    name: "agent",
    title: "Agent",
    tag: "Agent",
    desc: "多语言 Agent 网关与控制台，Go / Python 服务协同，工具调用与页面编排。",
    href: "/agent/",
    accent: "#a78bfa",
  },
  {
    name: "pages",
    title: "pages.lssl.work",
    tag: "Pages",
    desc: "Agent 上传的展示页托管：版本即上线，子域名隔离，适合看板与演示。",
    href: "https://pages.lssl.work/",
    accent: "#34d399",
  },
  {
    name: "lssl",
    title: "lssl.work",
    tag: "Site",
    desc: "主站入口与对外服务聚合，证书与路由由自建平台托管。",
    href: "https://lssl.work/",
    accent: "#f472b6",
  },
  {
    name: "pingpang",
    title: "乒乓球小程序",
    tag: "MiniApp",
    desc: "微信小程序：约球、对局与数据，独立仓库 my-pingpang。",
    href: "https://github.com/cool-sai/my-pingpang",
    accent: "#fbbf24",
  },
  {
    name: "news",
    title: "news.lssl.work",
    tag: "News",
    desc: "每日晨报站点，静态产物经 TLB 与证书自动续期对外提供。",
    href: "https://news.lssl.work/",
    accent: "#fb923c",
  },
  {
    name: "board",
    title: "Board",
    tag: "Board",
    desc: "刷题 / 通用看板 + 远程 MCP：浏览器只读，Agent 写数。",
    href: "https://lssl.work/board/",
    accent: "#fb7185",
  },
  {
    name: "img",
    title: "img.lssl.work",
    tag: "CDN",
    desc: "图片与静态资源域名，供站点与页面工程引用。",
    href: "https://img.lssl.work/",
    accent: "#60a5fa",
  },
];

function usePath(): [string, (to: string) => void] {
  const [path, setPath] = useState(() => window.location.pathname || "/");
  useEffect(() => {
    const onPop = () => setPath(window.location.pathname || "/");
    window.addEventListener("popstate", onPop);
    return () => window.removeEventListener("popstate", onPop);
  }, []);
  const navigate = (to: string) => {
    if (to === path) return;
    window.history.pushState({}, "", to);
    setPath(to);
  };
  return [path, navigate];
}

function pathToNav(path: string): NavKey {
  if (path === "/projects" || path.startsWith("/projects/")) return "projects";
  return "home";
}

function useSpotlight() {
  useEffect(() => {
    const el = document.documentElement;
    const onMove = (e: MouseEvent) => {
      el.style.setProperty("--spot-x", `${e.clientX}px`);
      el.style.setProperty("--spot-y", `${e.clientY}px`);
    };
    window.addEventListener("pointermove", onMove);
    return () => window.removeEventListener("pointermove", onMove);
  }, []);
}

function Shell({
  nav,
  onNav,
  children,
}: {
  nav: NavKey;
  onNav: (k: NavKey) => void;
  children: ReactNode;
}) {
  useSpotlight();
  return (
    <div className="relative min-h-dvh overflow-x-hidden bg-void text-ink">
      <div className="pointer-events-none fixed inset-0 -z-10">
        <div className="aurora aurora-a" />
        <div className="aurora aurora-b" />
        <div className="aurora aurora-c" />
        <div className="spotlight" />
        <div className="grid-overlay" />
        <div className="noise" />
      </div>

      <header className="mx-auto flex w-full max-w-6xl items-center justify-between gap-4 px-5 py-5 sm:px-8">
        <button
          type="button"
          onClick={() => onNav("home")}
          className="group flex items-center gap-3 border-0 bg-transparent p-0 text-left text-ink"
        >
          <span className="grid h-10 w-10 place-items-center rounded-xl border border-white/15 bg-white/5 font-mono text-sm font-semibold tracking-tight text-cyan shadow-[0_0_24px_rgba(34,211,238,0.25)]">
            极
          </span>
          <span>
            <span className="block text-sm font-semibold tracking-[0.18em] uppercase text-ink/90">
              Geek Lab
            </span>
            <span className="block text-xs text-mute">lssl · build in public</span>
          </span>
        </button>

        <nav className="flex items-center gap-1 rounded-full border border-white/10 bg-white/5 p-1 backdrop-blur-md">
          {(
            [
              ["home", "首页"],
              ["projects", "项目"],
            ] as const
          ).map(([key, label]) => (
            <button
              key={key}
              type="button"
              onClick={() => onNav(key)}
              className={
                "rounded-full border-0 px-4 py-1.5 text-sm transition " +
                (nav === key
                  ? "bg-white text-void shadow"
                  : "bg-transparent text-mute hover:text-ink")
              }
            >
              {label}
            </button>
          ))}
        </nav>
      </header>

      <div className="mx-auto w-full max-w-6xl px-5 sm:px-8">{children}</div>

      <footer className="mx-auto mt-16 flex w-full max-w-6xl flex-col items-center gap-2 px-5 py-8 text-xs text-mute sm:flex-row sm:justify-between sm:px-8">
        <span>© {new Date().getFullYear()} 极客 · 赛梁</span>
        <div className="flex flex-wrap items-center justify-center gap-3">
          <a
            href="https://beian.miit.gov.cn/"
            target="_blank"
            rel="noopener noreferrer"
            className="text-mute no-underline hover:text-ink"
          >
            京ICP备19043673号
          </a>
          <a
            href="https://beian.mps.gov.cn/#/query/webSearch?code=11010802033082"
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-1 text-mute no-underline hover:text-ink"
          >
            <img src="/gongan.png" alt="" width={14} height={14} />
            京公网安备11010802033082号
          </a>
        </div>
      </footer>
    </div>
  );
}

function Landing({ onProjects }: { onProjects: () => void }) {
  return (
    <main>
      <section className="relative pb-8 pt-10 sm:pb-14 sm:pt-16">
        <div className="hero-orb" aria-hidden />
        <p className="mb-4 inline-flex items-center gap-2 rounded-full border border-cyan/30 bg-cyan/10 px-3 py-1 text-xs font-medium tracking-wide text-cyan">
          <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-cyan" />
          Independent builder · Kitex · Agent · Infra
        </p>
        <h1 className="max-w-3xl text-4xl font-semibold leading-[1.08] tracking-tight sm:text-6xl">
          把基础设施
          <span className="bg-gradient-to-r from-cyan via-violet to-rose bg-clip-text text-transparent">
            {" "}
            亲手搭一遍
          </span>
        </h1>
        <p className="mt-5 max-w-2xl text-base leading-relaxed text-mute sm:text-lg">
          从 RPC、服务发现、可观测，到 SCM / 网关 / 证书 / 部署平台——不纸上谈兵。
          首页是展示；作品在「项目」页：minikitex、Agent、pages.lssl.work、乒乓球小程序等。
        </p>
        <div className="mt-8 flex flex-wrap items-center gap-3">
          <button
            type="button"
            onClick={onProjects}
            className="rounded-full border-0 bg-gradient-to-r from-cyan to-violet px-6 py-3 text-sm font-semibold text-void shadow-[0_0_40px_rgba(34,211,238,0.35)] transition hover:brightness-110"
          >
            浏览项目
          </button>
          <a
            href="https://github.com/cool-sai"
            target="_blank"
            rel="noopener noreferrer"
            className="rounded-full border border-white/15 bg-white/5 px-6 py-3 text-sm font-medium text-ink no-underline backdrop-blur hover:border-white/30"
          >
            GitHub
          </a>
          <a
            href="https://lssl.work/"
            target="_blank"
            rel="noopener noreferrer"
            className="rounded-full border border-white/15 bg-transparent px-6 py-3 text-sm font-medium text-mute no-underline hover:border-white/30 hover:text-ink"
          >
            lssl.work
          </a>
        </div>
      </section>

      <div className="marquee-wrap mb-10" aria-hidden>
        <div className="marquee">
          {[...SHOWCASE, ...SHOWCASE].map((s, i) => (
            <span key={`${s.name}-${i}`} className="marquee-chip" style={{ ["--c" as string]: s.accent }}>
              {s.title}
            </span>
          ))}
        </div>
      </div>

      <section className="grid gap-4 sm:grid-cols-3">
        {[
          {
            k: "01",
            t: "minikitex",
            d: "自研控制台与部署流水线，本地能跑、线上能发。",
          },
          {
            k: "02",
            t: "Agent + Pages",
            d: "Agent 写工具，pages.lssl.work 托管可视化结果。",
          },
          {
            k: "03",
            t: "端到端",
            d: "主站、小程序、证书与子域——按真实产品运维。",
          },
        ].map((c) => (
          <article
            key={c.k}
            className="glass-card rounded-2xl border border-white/10 bg-white/[0.03] p-5 backdrop-blur-sm transition hover:border-cyan/30 hover:bg-white/[0.05]"
          >
            <div className="font-mono text-xs text-cyan">{c.k}</div>
            <h2 className="mt-3 text-lg font-semibold tracking-tight">{c.t}</h2>
            <p className="mt-2 text-sm leading-relaxed text-mute">{c.d}</p>
          </article>
        ))}
      </section>

      <section className="mt-10 overflow-hidden rounded-3xl border border-white/10 bg-gradient-to-br from-white/[0.06] to-transparent p-6 sm:p-8">
        <div className="flex flex-col gap-6 sm:flex-row sm:items-end sm:justify-between">
          <div>
            <h2 className="text-2xl font-semibold tracking-tight">Stack</h2>
            <p className="mt-2 max-w-xl text-sm text-mute">
              Go / Kitex · Connect-RPC · React · Docker · Nginx TLB · MySQL · Prometheus /
              Loki / Jaeger · 微信小程序
            </p>
          </div>
          <button
            type="button"
            onClick={onProjects}
            className="self-start rounded-full border border-white/20 bg-transparent px-5 py-2 text-sm text-ink hover:border-cyan/50 hover:text-cyan"
          >
            查看全部项目 →
          </button>
        </div>
        <div className="mt-6 flex flex-wrap gap-2">
          {["Kitex", "Agent", "SCM", "TLB", "Pages", "Board", "Pingpang", "Observability"].map(
            (t) => (
              <span
                key={t}
                className="rounded-full border border-white/10 bg-black/20 px-3 py-1 font-mono text-xs text-mute"
              >
                {t}
              </span>
            ),
          )}
        </div>
      </section>
    </main>
  );
}

function ProjectsPage() {
  const [sites, setSites] = useState<Site[] | null>(null);
  const [err, setErr] = useState("");

  useEffect(() => {
    client.listSites({}).then(
      (d) => setSites(d.sites),
      (e: unknown) => setErr(e instanceof Error ? e.message : String(e)),
    );
  }, []);

  const apiExtra = useMemo(() => {
    const known = new Set(SHOWCASE.map((s) => s.name));
    return (sites ?? []).filter((s) => !known.has(s.name));
  }, [sites]);

  return (
    <main className="pb-10 pt-6 sm:pt-10">
      <div className="mb-8 max-w-2xl">
        <p className="mb-2 font-mono text-xs uppercase tracking-[0.2em] text-cyan">Projects</p>
        <h1 className="text-3xl font-semibold tracking-tight sm:text-5xl">在跑的作品</h1>
        <p className="mt-3 text-sm leading-relaxed text-mute sm:text-base">
          minikitex、Agent、pages.lssl.work、lssl.work、乒乓球小程序等。点卡片直接进入。
        </p>
      </div>

      {err ? (
        <p className="mb-4 rounded-xl border border-rose-500/30 bg-rose-500/10 px-4 py-3 text-sm text-rose-200">
          API：{err}（展示页仍可用下方精选列表）
        </p>
      ) : null}

      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        {SHOWCASE.map((s) => (
          <a
            key={s.name}
            href={s.href}
            target={s.href.startsWith("http") ? "_blank" : undefined}
            rel={s.href.startsWith("http") ? "noopener noreferrer" : undefined}
            className="group relative flex flex-col gap-3 overflow-hidden rounded-2xl border border-white/10 bg-white/[0.03] p-6 text-inherit no-underline backdrop-blur transition hover:-translate-y-0.5 hover:border-white/25 hover:bg-white/[0.06]"
          >
            <div
              className="pointer-events-none absolute -right-8 -top-8 h-32 w-32 rounded-full opacity-30 blur-3xl transition group-hover:opacity-50"
              style={{ background: s.accent }}
            />
            <div className="flex items-center justify-between gap-3">
              <span
                className="rounded-full border px-2.5 py-0.5 font-mono text-[11px] uppercase tracking-wide"
                style={{ borderColor: `${s.accent}55`, color: s.accent }}
              >
                {s.tag}
              </span>
              <span className="font-mono text-xs text-mute transition group-hover:text-ink">
                open ↗
              </span>
            </div>
            <h2 className="m-0 text-xl font-semibold tracking-tight">{s.title}</h2>
            <p className="m-0 flex-1 text-sm leading-relaxed text-mute">{s.desc}</p>
            <code className="font-mono text-xs" style={{ color: s.accent }}>
              {s.href}
            </code>
          </a>
        ))}

        {apiExtra.map((s) => (
          <a
            key={s.name}
            href={s.href}
            className="flex flex-col gap-3 rounded-2xl border border-white/10 bg-white/[0.03] p-6 text-inherit no-underline hover:border-cyan/30"
          >
            <h2 className="m-0 text-xl font-semibold tracking-tight">{s.title}</h2>
            <p className="m-0 flex-1 text-sm leading-relaxed text-mute">{s.desc}</p>
            <code className="font-mono text-xs text-cyan">{s.href}</code>
          </a>
        ))}
      </div>

      {!err && !sites ? (
        <p className="mt-4 text-sm text-mute">同步控制台站点列表…</p>
      ) : null}
    </main>
  );
}

export default function App() {
  const [path, navigate] = usePath();
  const nav = pathToNav(path);

  return (
    <Shell
      nav={nav}
      onNav={(k) => navigate(k === "projects" ? "/projects" : "/")}
    >
      {nav === "projects" ? <ProjectsPage /> : <Landing onProjects={() => navigate("/projects")} />}
    </Shell>
  );
}
