import { copyFileSync, mkdirSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

// Files that need a stable public URL (favicon, og:image). Everything else is
// imported straight from docs/ so Astro can optimize it.
const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const docs = resolve(root, '../docs');
const pub = resolve(root, 'public');

const files = {
  'brand/mark.svg': 'favicon.svg',
  'social-preview.png': 'social-preview.png',
};

mkdirSync(pub, { recursive: true });
for (const [from, to] of Object.entries(files)) {
  copyFileSync(resolve(docs, from), resolve(pub, to));
}
