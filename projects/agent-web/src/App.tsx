import { useEffect, useState } from "react";

type Hello = { service?: string; msg?: string };

export default function App() {
  const [hello, setHello] = useState<Hello | null>(null);
  const [err, setErr] = useState("");

  useEffect(() => {
    fetch("/api/hello")
      .then(async (res) => {
        if (!res.ok) {
          throw new Error(res.statusText);
        }
        return res.json() as Promise<Hello>;
      })
      .then(setHello, (e: unknown) => setErr(e instanceof Error ? e.message : String(e)));
  }, []);

  return (
    <div style={{ fontFamily: "ui-sans-serif, system-ui, sans-serif", padding: 48, maxWidth: 640 }}>
      <p style={{ color: "#64748b", margin: 0 }}>agent-web</p>
      <h1 style={{ marginTop: 8 }}>Agent</h1>
      <p style={{ color: "#475569" }}>经 TLB 访问时，/ 打到 agent-web，/api 打到 agent-server。</p>
      {err ? <p style={{ color: "#b91c1c" }}>{err}</p> : null}
      {hello ? (
        <pre style={{ background: "#f1f5f9", padding: 16, borderRadius: 8 }}>
          {JSON.stringify(hello, null, 2)}
        </pre>
      ) : (
        <p style={{ color: "#94a3b8" }}>请求 /api/hello …</p>
      )}
    </div>
  );
}
