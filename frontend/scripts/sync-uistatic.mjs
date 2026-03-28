import { cpSync, existsSync, mkdirSync, rmSync } from "fs";
import { dirname, resolve } from "path";
import { fileURLToPath } from "url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(__dirname, "../..");
const src = resolve(repoRoot, "frontend/dist");
const dest = resolve(repoRoot, "backend/cmd/wails/dist");

if (!existsSync(src)) {
  console.error("sync-uistatic: frontend/dist not found (run vite build first).");
  process.exit(1);
}

rmSync(dest, { recursive: true, force: true });
mkdirSync(dirname(dest), { recursive: true });
cpSync(src, dest, { recursive: true });
console.log("sync-uistatic: copied UI for Wails embed ->", dest);
