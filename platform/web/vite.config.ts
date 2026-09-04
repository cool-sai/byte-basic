import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    host: true,
    port: 5173,
    allowedHosts: [".ls-byte-basic.com"],
    proxy: {
      "/api": {
        target: "http://127.0.0.1:8081",
        changeOrigin: true,
        timeout: 0,
        proxyTimeout: 0,
      },
      "/platform.v1.PlatformService": {
        target: "http://127.0.0.1:8081",
        changeOrigin: true,
        timeout: 0,
        proxyTimeout: 0,
      },
    },
  },
});
