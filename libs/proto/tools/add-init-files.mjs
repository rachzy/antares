import { mkdirSync, readdirSync, statSync, writeFileSync } from "node:fs";
import { join, resolve } from "node:path";

/** Drop an empty __init__.py into every directory under each root. */
function markPackages(root) {
  mkdirSync(root, { recursive: true });
  writeFileSync(join(root, "__init__.py"), "");
  for (const entry of readdirSync(root)) {
    const path = join(root, entry);
    if (entry !== "__pycache__" && statSync(path).isDirectory()) markPackages(path);
  }
}

for (const arg of process.argv.slice(2)) markPackages(resolve(arg));
