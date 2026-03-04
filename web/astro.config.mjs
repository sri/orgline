import { defineConfig } from "astro/config";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const rootDir = dirname(fileURLToPath(import.meta.url));
const backendURL = process.env.ORGLINE_DEV_BACKEND_URL ?? "http://127.0.0.1:8080";

export default defineConfig({
  outDir: resolve(rootDir, "../internal/frontend/dist/site"),
  server: {
    host: "127.0.0.1",
    port: 5173
  },
  vite: {
    server: {
      proxy: {
        "/api": {
          target: backendURL,
          changeOrigin: true
        }
      }
    }
  }
});
