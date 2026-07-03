
import { defineConfig, loadEnv } from "vite";
import react from "@vitejs/plugin-react-swc";
import path from "path";
import { readFileSync } from "fs";

const { version: appVersion } = JSON.parse(
  readFileSync(path.resolve(__dirname, "./package.json"), "utf-8")
);

// https://vitejs.dev/config/
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '');
  const target = env.VITE_API_TARGET || "http://localhost:8081";

  console.log(`[Vite] Proxying /api to: ${target}`);

  return {
    server: {
      host: "0.0.0.0",
      port: 3003,
      proxy: {
        "/api": {
          target: target,
          changeOrigin: true,
          secure: false,
        },
      },
    },
    plugins: [
      react(),
    ],
    resolve: {
      alias: {
        "@": path.resolve(__dirname, "./src"),
      },
    },
    // Baked into the bundle at build time — lets Login.tsx show the frontend
    // version alongside the backend's live /api/health version, so a stale
    // (cached/not-yet-redeployed) frontend is visually distinguishable from
    // a stale backend instead of both looking like "the app didn't update".
    define: {
      __APP_VERSION__: JSON.stringify(appVersion),
    },
  };
});