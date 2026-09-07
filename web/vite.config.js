import { defineConfig } from 'vite';

export default defineConfig({
  // Deployed to GitHub Pages at https://ryuken25.github.io/audioprep/
  base: '/audioprep/',
  build: {
    outDir: 'dist',
    target: 'es2022',
    emptyOutDir: true,
  },
  // Keep the ffmpeg packages out of dependency pre-bundling so the module worker
  // (`new Worker(new URL('./worker.js', import.meta.url))`) resolves correctly in dev.
  optimizeDeps: {
    exclude: ['@ffmpeg/ffmpeg', '@ffmpeg/util'],
  },
  server: {
    port: 5173,
  },
});
