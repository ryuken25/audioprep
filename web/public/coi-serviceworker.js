/*
 * Cross-origin isolation via a service worker.
 *
 * The multi-threaded ffmpeg core needs SharedArrayBuffer, and a browser only
 * hands that out to a cross-origin-isolated page: one served with
 *   Cross-Origin-Opener-Policy: same-origin
 *   Cross-Origin-Embedder-Policy: require-corp
 *
 * Vercel sets those headers directly (see vercel.json) and this file does
 * nothing there. A plain static host like GitHub Pages cannot set headers at
 * all, so this worker intercepts the page's own responses and adds them, which
 * is enough to flip crossOriginIsolated on. Same-origin only: we never touch a
 * third-party response, and the app makes no third-party requests anyway.
 *
 * Registration happens in index.html before the app loads. The first visit
 * installs the worker and reloads once; from then on the page is isolated.
 * If anything here fails the app simply stays single-threaded.
 */

if (typeof window === 'undefined') {
  // ---- service worker side ----
  self.addEventListener('install', () => self.skipWaiting());
  self.addEventListener('activate', (e) => e.waitUntil(self.clients.claim()));

  self.addEventListener('fetch', (event) => {
    const req = event.request;
    // Range requests must pass through untouched or media seeking breaks.
    if (req.cache === 'only-if-cached' && req.mode !== 'same-origin') return;

    event.respondWith(
      fetch(req)
        .then((res) => {
          if (res.status === 0) return res; // opaque, nothing to rewrite
          const headers = new Headers(res.headers);
          headers.set('Cross-Origin-Embedder-Policy', 'require-corp');
          headers.set('Cross-Origin-Opener-Policy', 'same-origin');
          // Our own assets must be embeddable by the isolated page.
          headers.set('Cross-Origin-Resource-Policy', 'same-origin');
          return new Response(res.body, { status: res.status, statusText: res.statusText, headers });
        })
        .catch((err) => {
          console.error('[coi] fetch failed', err);
          throw err;
        }),
    );
  });
} else {
  // ---- page side ----
  (() => {
    // Already isolated (Vercel's headers, or a previous run of this worker).
    if (window.crossOriginIsolated) return;
    if (!window.isSecureContext || !('serviceWorker' in navigator)) return;

    // Reload at most once, so a browser that refuses to isolate cannot loop.
    const KEY = 'kxc.coiReloaded';
    const alreadyReloaded = (() => {
      try { return sessionStorage.getItem(KEY) === '1'; } catch { return true; }
    })();

    navigator.serviceWorker
      .register(new URL('coi-serviceworker.js', document.currentScript ? document.currentScript.src : location.href))
      .then((reg) => {
        reg.addEventListener('updatefound', () => window.location.reload());
        if (reg.active && !navigator.serviceWorker.controller && !alreadyReloaded) {
          try { sessionStorage.setItem(KEY, '1'); } catch { /* private mode */ }
          window.location.reload();
        }
      })
      .catch((err) => console.warn('[coi] registration failed, staying single-threaded', err));
  })();
}
