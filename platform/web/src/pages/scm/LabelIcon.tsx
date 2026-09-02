export default function LabelIcon({ label, size = 20 }: { label?: string; size?: number }) {
  if (label === "node") {
    return (
      <svg width={size} height={size} viewBox="0 0 24 24" aria-label="node" className="shrink-0">
        <path
          fill="#5FA04E"
          d="M12 1.7 21.5 7v10L12 22.3 2.5 17V7L12 1.7zm0 2.3L4.7 8.1v7.8L12 20l7.3-4.1V8.1L12 4z"
        />
        <path fill="#5FA04E" d="M11.1 8.2h1.8c1.5 0 2.4.7 2.4 1.9 0 .9-.5 1.5-1.3 1.7l1.6 2.4h-1.5l-1.4-2.2h-.6V14.2h-1V8.2zm1.7 3c.7 0 1.1-.3 1.1-.9s-.4-.9-1.1-.9h-.7v1.8h.7z" />
      </svg>
    );
  }
  if (label === "golang") {
    return (
      <svg width={size} height={size} viewBox="0 0 24 24" aria-label="golang" className="shrink-0">
        <rect width="24" height="24" rx="6" fill="#00ADD8" />
        <text x="12" y="16.5" textAnchor="middle" fill="#fff" fontSize="10" fontFamily="ui-sans-serif, system-ui, sans-serif" fontWeight="700">
          Go
        </text>
      </svg>
    );
  }
  return null;
}
