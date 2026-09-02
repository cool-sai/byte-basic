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

function HelloPanel({ title, path }: { title: string; path: string }) {
  const [hello, setHello] = useState<Hello | null>(null);
  const [err, setErr] = useState("");

  useEffect(() => {
    loadHello(path).then(setHello, (e: unknown) => setErr(e instanceof Error ? e.message : String(e)));
  }, [path]);

  return (
    <div>
      <p style={{ color: "#64748b", margin: "0 0 8px" }}>
        {title} · {path}
      </p>
      {err ? <p style={{ color: "#b91c1c" }}>{err}</p> : null}
      {hello ? (
        <pre style={{ background: "#f1f5f9", padding: 16, borderRadius: 8, margin: 0 }}>
          {JSON.stringify(hello, null, 2)}
        </pre>
      ) : err ? null : (
        <p style={{ color: "#94a3b8" }}>请求中…</p>
      )}
    </div>
  );
}

export default function App() {
  return (
    <div style={{ fontFamily: "ui-sans-serif, system-ui, sans-serif", padding: 48, maxWidth: 640 }}>
      <p style={{ color: "#64748b", margin: 0 }}>agent-web</p>
      <h1 style={{ marginTop: 8 }}>Agent</h1>
      <p style={{ color: "#475569" }}>
        / 打到 agent-web，/api 打到 agent-server，/python 打到 agent-python。
      </p>
      <div style={{ display: "flex", flexDirection: "column", gap: 24, marginTop: 24 }}>
        <HelloPanel title="agent-server" path="/api/hello" />
        <HelloPanel title="agent-python" path="/python/api/hello" />
      </div>
    </div>
  );
}
