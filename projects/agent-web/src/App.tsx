import { useEffect, useState } from "react";

type Hello = { service?: string; msg?: string };

function loadHello(path: string): Promise<Hello | null> {
  return fetch(path).then(async (res) => {
    const body = (await res.json()) as { error?: string; data?: Hello };
    if (body.error) {
      throw new Error(body.error);
    }
    if (!res.ok) {
      throw new Error(res.statusText);
    }
    return body.data ?? null;
  });
}

function HelloPanel({
  title,
  path,
  lang,
}: {
  title: string;
  path: string;
  lang: "go" | "py";
}) {
  const [hello, setHello] = useState<Hello | null>(null);
  const [err, setErr] = useState("");

  useEffect(() => {
    loadHello(path).then(setHello, (e: unknown) => setErr(e instanceof Error ? e.message : String(e)));
  }, [path]);

  let pill = "wait";
  let pillText = "请求中";
  if (err) {
    pill = "bad";
    pillText = "失败";
  } else if (hello) {
    pill = "ok";
    pillText = "在线";
  }

  return (
    <article className="card">
      <header className="card-hd">
        <span className={"lang " + lang}>{lang === "go" ? "Go" : "Py"}</span>
        <div>
          <h2>{title}</h2>
          <code className="path mono">{path}</code>
        </div>
        <span className={"pill " + pill}>{pillText}</span>
      </header>
      {err ? <p className="msg bad">{err}</p> : null}
      {hello ? (
        <pre className="payload">{JSON.stringify(hello, null, 2)}</pre>
      ) : err ? null : (
        <p className="msg">正在请求 {path}</p>
      )}
    </article>
  );
}

export default function App() {
  return (
    <div className="page">
      <header className="top">
        <div className="brand">
          <div className="mark">A</div>
          <div>
            <h1>Agent</h1>
            <p>agent-web</p>
          </div>
        </div>
        <div className="routes">
          <span className="chip">
            <b>/</b> web
          </span>
          <span className="chip">
            <b>/api</b> server
          </span>
          <span className="chip">
            <b>/python</b> python
          </span>
        </div>
      </header>
      <div className="grid">
        <HelloPanel title="agent-server" path="/api/hello" lang="go" />
        <HelloPanel title="agent-python" path="/python/api/hello" lang="py" />
      </div>
    </div>
  );
}
