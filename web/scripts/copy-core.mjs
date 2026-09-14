// Copies both ffmpeg.wasm cores into public/ffmpeg/ so the app self-hosts them
// (no CDN, and a CDN's MIME types cannot break us).
//
// - core/    single-threaded. Works anywhere, including a plain static host.
// - core-mt/ multi-threaded. Needs SharedArrayBuffer, which needs the page to be
//            cross-origin isolated (COOP + COEP). Vercel sets those headers in
//            vercel.json; elsewhere a service worker can add them, and if
//            neither is present the app falls back to the single-threaded core.
import { copyFileSync, existsSync, mkdirSync, statSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const dstDir = join(root, 'public', 'ffmpeg');

const bundles = [
  { pkg: '@ffmpeg/core', sub: '', files: ['ffmpeg-core.js', 'ffmpeg-core.wasm'] },
  // The MT core ships a worker file too; it must sit next to the core js.
  { pkg: '@ffmpeg/core-mt', sub: 'mt', files: ['ffmpeg-core.js', 'ffmpeg-core.wasm', 'ffmpeg-core.worker.js'] },
];

let copied = 0;
for (const { pkg, sub, files } of bundles) {
  const srcDir = join(root, 'node_modules', ...pkg.split('/'), 'dist', 'esm');
  if (!existsSync(srcDir)) {
    // postinstall can run before the dependency tree is fully written (npm ci
    // ordering); the build script runs this again, so warn rather than fail.
    console.warn(`[copy-core] ${srcDir} not found, skipping`);
    continue;
  }
  const out = sub ? join(dstDir, sub) : dstDir;
  mkdirSync(out, { recursive: true });
  for (const f of files) {
    const src = join(srcDir, f);
    if (!existsSync(src)) {
      console.warn(`[copy-core] ${pkg}/${f} missing, skipping`);
      continue;
    }
    copyFileSync(src, join(out, f));
    const mb = (statSync(join(out, f)).size / 1048576).toFixed(1);
    console.log(`[copy-core] ${pkg} ${f} -> public/ffmpeg/${sub ? sub + '/' : ''} (${mb} MB)`);
    copied++;
  }
}

if (!copied) {
  console.warn('[copy-core] nothing copied (run "npm run build" to retry)');
}
