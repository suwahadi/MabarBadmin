import { createHash } from "node:crypto";
import { readFileSync, writeFileSync, renameSync, readdirSync, unlinkSync } from "node:fs";
import { join } from "node:path";

// Renames built assets to content-hashed names so they can be cached as immutable for a year,
// and writes manifest.json for the Go templates.
const root = new URL("../web/static/", import.meta.url).pathname;
const manifest = {};
for (const [dir, file] of [["css", "app.css"], ["js", "app.js"]]) {
  const folder = join(root, dir);
  for (const old of readdirSync(folder)) {
    if (/^app\.[0-9a-f]{10}\.(css|js)$/.test(old)) unlinkSync(join(folder, old));
  }
  const src = join(folder, file);
  const hash = createHash("sha256").update(readFileSync(src)).digest("hex").slice(0, 10);
  const name = file.replace(/\.(css|js)$/, `.${hash}.$1`);
  renameSync(src, join(folder, name));
  manifest[`${dir}/${file}`] = `${dir}/${name}`;
}
writeFileSync(join(root, "manifest.json"), JSON.stringify(manifest, null, 2) + "\n");
console.log(manifest);
