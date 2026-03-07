import { cpSync, mkdirSync, readdirSync, rmSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const sourceDir = resolve(__dirname, "../dist/dashboard/browser");
const publicDir = resolve(__dirname, "../../../public");

mkdirSync(publicDir, { recursive: true });

for (const entry of readdirSync(publicDir, { withFileTypes: true })) {
    if (entry.name === "embed.go") {
        continue;
    }

    rmSync(resolve(publicDir, entry.name), { recursive: true, force: true });
}

for (const entry of readdirSync(sourceDir, { withFileTypes: true })) {
    cpSync(resolve(sourceDir, entry.name), resolve(publicDir, entry.name), {
        recursive: true,
        force: true
    });
}
