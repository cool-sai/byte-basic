import { useEffect, useState } from "react";
import { client } from "./api";
import type { Site } from "./gen/home/v1/home_pb";

export default function App() {
  const [sites, setSites] = useState<Site[] | null>(null);
  const [err, setErr] = useState("");

  useEffect(() => {
    client.listSites({}).then(
      (d) => setSites(d.sites),
      (e: unknown) => setErr(e instanceof Error ? e.message : String(e)),
    );
  }, []);

  return (
    <div className="flex min-h-dvh flex-col bg-page font-sans text-ink">
      <header className="px-6 pt-14 pb-8 text-center sm:pt-20">
        <h1 className="m-0 text-5xl font-semibold tracking-tight sm:text-6xl">极客</h1>
      </header>

      <main className="mx-auto w-full max-w-[880px] flex-1 px-6 pb-16">
        {err ? <p className="mb-4 text-sm text-rose-600">{err}</p> : null}
        {!err && !sites ? <p className="mb-4 text-sm text-mute">加载中</p> : null}
        <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
          {(sites ?? []).map((s) => (
            <a
              key={s.name}
              href={s.href}
              className="flex flex-col gap-2 rounded-xl border border-line bg-white p-5 text-inherit no-underline shadow-sm hover:border-accent/35 hover:shadow-md"
            >
              <h2 className="m-0 text-lg font-medium tracking-tight">{s.title}</h2>
              <p className="m-0 flex-1 text-sm leading-relaxed text-mute">{s.desc}</p>
              <code className="font-mono text-xs text-accent">{s.href}</code>
            </a>
          ))}
        </div>
      </main>

      <footer className="flex flex-col items-center gap-2 px-4 py-5 sm:flex-row sm:gap-4">
        <a
          href="https://beian.miit.gov.cn/"
          target="_blank"
          rel="noopener noreferrer"
          className="text-sm text-mute no-underline hover:text-ink hover:underline"
        >
          京ICP备19043673号
        </a>
        <a
          href="https://beian.mps.gov.cn/#/query/webSearch?code=11010802033082"
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex items-center gap-1 text-sm text-mute no-underline hover:text-ink hover:underline"
        >
          <img src="/gongan.png" alt="" width={16} height={16} />
          京公网安备11010802033082号
        </a>
      </footer>
    </div>
  );
}
