import fs from 'node:fs'
import path from 'node:path'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { defineConfig } from 'vite'

// 一次只构建一个页面目标：PAGE_TARGET=news/daily → src/pages/news/daily → dist/news/daily
// 由 scripts/build.mjs 设置；直接 `vite` 开发时也用同一个变量。
const target = (process.env.PAGE_TARGET || '').replace(/^\/+|\/+$/g, '')
if (!/^[a-z0-9][a-z0-9_-]*(\/[a-z0-9][a-z0-9_-]*){0,3}$/.test(target)) {
  throw new Error(`PAGE_TARGET 不合法: "${target}"（例：news/daily）`)
}
const root = path.resolve(import.meta.dirname, 'src/pages', target)
const input = Object.fromEntries(
  ['index', 'detail'].filter((n) => fs.existsSync(path.join(root, `${n}.html`))).map((n) => [n, path.join(root, `${n}.html`)]),
)
if (!input.index) throw new Error(`${root}/index.html 不存在`)

export default defineConfig({
  root,
  // 相对路径，上传到任意 put_page 路径都能用；home 例外：它在 pages 域名根路径 / 展示，
  // ./assets 会解析成 /assets，所以用绝对路径 /home/。
  base: target === 'home' ? '/home/' : './',
  envDir: import.meta.dirname,
  publicDir: false,
  plugins: [react(), tailwindcss()],
  resolve: { alias: { '@': path.resolve(import.meta.dirname, './src') } },
  // 本地 dev：同域 /api/data → board /data（与线上 nginx 一致）
  server: {
    proxy: {
      '/api/data': {
        target: process.env.VITE_PROXY_BOARD || 'http://localhost:18090',
        changeOrigin: true,
        rewrite: (p) => p.replace(/^\/api/, ''),
      },
    },
  },
  build: {
    outDir: path.resolve(import.meta.dirname, 'dist', target),
    emptyOutDir: true,
    assetsDir: 'assets',
    modulePreload: { polyfill: false },
    rollupOptions: { input },
  },
})
