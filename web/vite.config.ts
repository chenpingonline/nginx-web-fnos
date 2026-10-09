import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import { readFileSync } from "node:fs";

const manifest = readFileSync(new URL("../packaging/fnos/manifest", import.meta.url), "utf8");
const versions = [...manifest.matchAll(/^version[\t ]*=[\t ]*([0-9]+\.[0-9]+\.[0-9]+)[\t ]*\r?$/gm)];
if (versions.length !== 1) throw new Error("manifest must contain exactly one version in major.minor.patch format");

const permissionMode = process.env.FNPROXY_PERMISSION_MODE || "standard";
const frontendMode = process.env.FNPROXY_FRONTEND_MODE || "fnos";
if (!["fnos", "standalone"].includes(frontendMode)) throw new Error("Invalid frontend mode");
if (!["standard", "full-ports"].includes(permissionMode)) throw new Error("Invalid permission mode");

export default defineConfig({
  base: "./",
  plugins: [vue()],
  define: { __APP_VERSION__: JSON.stringify(versions[0][1]), __MIN_LISTEN_PORT__: permissionMode === "full-ports" ? 1 : 1024, __STANDALONE__: frontendMode === "standalone" },
  build: { outDir: "dist", emptyOutDir: true },
});
