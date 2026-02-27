import { defineConfig } from "astro/config";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const rootDir = dirname(fileURLToPath(import.meta.url));

export default defineConfig({
  outDir: resolve(rootDir, "../internal/frontend/dist/site"),
  server: {
    host: "127.0.0.1",
    port: 5173
  }
});
