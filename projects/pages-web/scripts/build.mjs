// 用法：npm run build -- shell [--env local|prod]
// 只构建一个页面目标，输出到 dist/<target>/。
import { spawnSync } from "node:child_process";
import { parseArgs } from "./args.mjs";

const { target, env } = parseArgs(process.argv.slice(2));
// prod：不设 origin → 同域 /api/data
// local：默认直连本地 board 的 /data；也可自设 VITE_API_BASE / VITE_API_ORIGIN，或走 vite /api/data 代理
const childEnv = { ...process.env, PAGE_TARGET: target };
if (!childEnv.VITE_API_BASE && !childEnv.VITE_API_ORIGIN) {
  if (env === "local") childEnv.VITE_API_ORIGIN = "http://localhost:18090";
  // prod: leave unset → /api/data
}
const apiLabel = childEnv.VITE_API_BASE || (childEnv.VITE_API_ORIGIN ? `${childEnv.VITE_API_ORIGIN.replace(/\/+$/, "")}/data` : "/api/data");
console.log(`构建 ${target}（env=${env}，API=${apiLabel}）`);
const r = spawnSync("npx", ["tsc", "-b"], { stdio: "inherit" });
if (r.status !== 0) process.exit(r.status ?? 1);
const v = spawnSync("npx", ["vite", "build"], {
  stdio: "inherit",
  env: childEnv,
});
process.exit(v.status ?? 1);
