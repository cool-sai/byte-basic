// 用法：BOARD_TOKEN=xxx npm run publish -- news/daily --env local|prod
// 先构建，再把 dist/<target>/ 的全部文件通过 board MCP 的 put_page 上传为新版本（立即上线，可 rollback_page 回滚）。
// token 只从环境变量 BOARD_TOKEN 读取，绝不打印。
import fs from "node:fs";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { parseArgs } from "./args.mjs";

const { target, env } = parseArgs(process.argv.slice(2));
const MCP = { local: "http://localhost:18090/mcp", prod: "https://lssl.work/board/mcp" }[env];
const token = (process.env.BOARD_TOKEN || "").trim();
if (!token) { console.error("缺少环境变量 BOARD_TOKEN"); process.exit(1); }

// board 的上传限制（见 board/content.go）：最多 64 个文件、总计 4 MiB、文本内容、必须有 index.html
const MAX_FILES = 64, MAX_BYTES = 4 << 20, NAME_RE = /^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$/;
const TEXT_EXT = new Set([".html", ".js", ".mjs", ".css", ".json", ".svg", ".txt", ".map"]);

if (!process.argv.includes("--no-build")) {
  const b = spawnSync("node", ["scripts/build.mjs", target, "--env", env], { stdio: "inherit" });
  if (b.status !== 0) process.exit(b.status ?? 1);
}

const dir = path.resolve("dist", target);
const files = {};
let total = 0;
for (const f of fs.readdirSync(dir, { recursive: true })) {
  const full = path.join(dir, f);
  if (fs.statSync(full).isDirectory()) continue;
  const name = f.split(path.sep).join("/");
  if (name.endsWith(".map")) continue;
  if (!TEXT_EXT.has(path.extname(name))) { console.error(`不支持的文件类型（board 只存文本）：${name}`); process.exit(1); }
  if (!name.split("/").every((s) => NAME_RE.test(s))) { console.error(`文件名不合法：${name}`); process.exit(1); }
  files[name] = fs.readFileSync(full, "utf8");
  total += Buffer.byteLength(files[name]);
}
const count = Object.keys(files).length;
console.log(`待上传 ${count} 个文件，共 ${(total / 1024).toFixed(1)} KiB → ${env}:${target}`);
if (!files["index.html"]) { console.error("缺少 index.html"); process.exit(1); }
if (count > MAX_FILES || total > MAX_BYTES) { console.error(`超出 board 限制（${MAX_FILES} 个文件 / 4 MiB）`); process.exit(1); }

const res = await fetch(MCP, {
  method: "POST",
  headers: { "Content-Type": "application/json", Accept: "application/json, text/event-stream", Authorization: `Bearer ${token}` },
  body: JSON.stringify({ jsonrpc: "2.0", id: 1, method: "tools/call", params: { name: "put_page", arguments: { path: target, files } } }),
});
const text = await res.text();
if (!res.ok) { console.error(`上传失败 HTTP ${res.status}: ${text.slice(0, 300)}`); process.exit(1); }
const json = JSON.parse(text.replace(/^data: /m, "").trim().split("\n").find((l) => l.startsWith("{")) || text);
const out = json.result?.content?.[0]?.text;
if (json.error || json.result?.isError) { console.error("上传失败：", json.error?.message || out); process.exit(1); }
console.log("已发布：", out);
