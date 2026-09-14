import { defineConfig } from 'vite';
import { statSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

// Size of the wasm core we ship. The loading bar needs a real total because
// GitHub Pages gzips the file and Content-Length then reports the compressed
// size (about 10 MB for a 32 MB core). Zero if the package is not installed.
function coreWasmBytes(pkg = '@ffmpeg/core') {
  try {
    const p = fileURLToPath(new URL(`./node_modules/${pkg}/dist/esm/ffmpeg-core.wasm`, import.meta.url));
    return statSync(p).size;
  } catch {
    return 0;
  }
}

// Two hosts: GitHub Pages serves the site under /audioprep/, Vercel at the
// root. Vercel sets VERCEL=1 in its build environment; VITE_BASE overrides
// both for anything else.
const base = process.env.VITE_BASE || (process.env.VERCEL ? '/' : '/audioprep/');

const COI_HEADERS = {
  'Cross-Origin-Opener-Policy': 'same-origin',
  'Cross-Origin-Embedder-Policy': 'require-corp',
  'Cross-Origin-Resource-Policy': 'same-origin',
};

export default defineConfig({
  base,
  build: {
    outDir: 'dist',
    target: 'es2022',
    emptyOutDir: true,
  },
  define: {
    __CORE_WASM_BYTES__: JSON.stringify(coreWasmBytes()),
    __CORE_MT_WASM_BYTES__: JSON.stringify(coreWasmBytes('@ffmpeg/core-mt')),
  },
  // Keep the ffmpeg packages out of dependency pre-bundling so the module worker
  // (`new Worker(new URL('./worker.js', import.meta.url))`) resolves correctly in dev.
  optimizeDeps: {
    exclude: ['@ffmpeg/ffmpeg', '@ffmpeg/util'],
  },
  // The multi-threaded core needs SharedArrayBuffer, which needs the page to be
  // cross-origin isolated. Vercel sets these in vercel.json; set them locally
  // too so `npm run dev` and `npm run preview` behave like production.
  server: {
    port: 5173,
    headers: COI_HEADERS,
  },
  preview: {
    port: 4173,
    headers: COI_HEADERS,
  },
});
