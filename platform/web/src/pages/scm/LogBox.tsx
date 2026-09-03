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
      className="logbox"
    >
      {text || (live ? "启动中…" : "无日志")}
    </pre>
  );
}
