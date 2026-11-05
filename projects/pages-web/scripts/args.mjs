export function parseArgs(argv) {
  let target = "";
  let env = "prod";
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    if (a === "--env") env = argv[++i];
    else if (a.startsWith("--env=")) env = a.slice(6);
    else if (!a.startsWith("-")) target = a.replace(/^\/+|\/+$/g, "");
  }
  if (!target) throw new Error("缺少页面目标，例如：news/daily");
  if (!["local", "prod"].includes(env)) throw new Error(`--env 只能是 local 或 prod：${env}`);
  return { target, env };
}
