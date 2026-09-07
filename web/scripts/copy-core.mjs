// Copies the single-threaded ffmpeg.wasm core into public/ffmpeg/ so the app
// can self-host it (no CDN, works on GitHub Pages without COOP/COEP headers).
import { copyFileSync, existsSync, mkdirSync, statSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const srcDir = join(root, 'node_modules', '@ffmpeg', 'core', 'dist', 'esm');
const dstDir = join(root, 'public', 'ffmpeg');
const files = ['ffmpeg-core.js', 'ffmpeg-core.wasm'];

if (!existsSync(srcDir)) {
  // postinstall can run before the dependency tree is fully written (e.g. `npm ci` ordering);
  // the build script runs this again, so just warn here.
  console.warn(`[copy-core] ${srcDir} not found, skipping (run "npm run build" to retry)`);
  process.exit(0);
}

mkdirSync(dstDir, { recursive: true });
for (const f of files) {
  const src = join(srcDir, f);
  const dst = join(dstDir, f);
  copyFileSync(src, dst);
  const mb = (statSync(dst).size / 1048576).toFixed(1);
  console.log(`[copy-core] ${f} -> public/ffmpeg/ (${mb} MB)`);
}
