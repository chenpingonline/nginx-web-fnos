import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import { readFileSync } from "node:fs";

const manifest = readFileSync(new URL("../packaging/fnos/manifest", import.meta.url), "utf8");
const versions = [...manifest.matchAll(/^version[\t ]*=[\t ]*([0-9]+\.[0-9]+\.[0-9]+)[\t ]*\r?$/gm)];
if (versions.length !== 1) throw new Error("manifest must contain exactly one version in major.minor.patch format");

const permissionMode = process.env.FNPROXY_PERMISSION_MODE || "standard";
if (!["standard", "full-ports"].includes(permissionMode)) throw new Error("Invalid permission mode");

export default defineConfig({
  base: "./",
  plugins: [vue()],
  define: { __APP_VERSION__: JSON.stringify(versions[0][1]), __MIN_LISTEN_PORT__: permissionMode === "full-ports" ? 1 : 1024 },
  build: { outDir: "dist", emptyOutDir: true },
});
