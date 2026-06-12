import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { fileURLToPath, URL } from "node:url";

// FluxTorrent UI → static assets embedded into the Go binary (SPEC §3).
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: { "@": fileURLToPath(new URL("./src", import.meta.url)) },
  },
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
  server: {
    // dev proxy so `npm run dev` talks to a local backend on :7001
    proxy: {
      "/api": { target: "http://localhost:7001", changeOrigin: true, ws: true },
      "/stream": { target: "http://localhost:7001", changeOrigin: true },
    },
  },
});
