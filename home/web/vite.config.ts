import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    host: true,
    port: 5175,
    allowedHosts: [".ls-byte-basic.com"],
    proxy: {
      "/api": {
        target: "http://127.0.0.1:8082",
        changeOrigin: true,
      },
      "/home.v1.HomeService": {
        target: "http://127.0.0.1:8082",
        changeOrigin: true,
      },
    },
  },
});
