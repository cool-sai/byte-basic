import { useEffect, useRef } from "react";

export default function LogBox({ text, live }: { text: string; live?: boolean }) {
  const ref = useRef<HTMLPreElement>(null);
  useEffect(() => {
    const el = ref.current;
    if (live && el) {
      el.scrollTop = el.scrollHeight;
    }
  }, [text, live]);
  return (
    <pre
      ref={ref}
      className="m-0 max-h-[28rem] overflow-auto whitespace-pre-wrap bg-slate-900 p-3 font-mono text-xs text-slate-100"
    >
      {text || (live ? "启动中…" : "无日志")}
    </pre>
  );
}
