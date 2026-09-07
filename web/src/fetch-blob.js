// Download a file into a blob: URL with progress, on servers that compress.
//
// @ffmpeg/util's toBlobURL(progress = true) compares Content-Length with the
// bytes it pulled from the stream. When the server gzips the response (GitHub
// Pages serves ffmpeg-core.wasm as 10 MB on the wire for 32 MB of content),
// Content-Length is the compressed size and the stream yields decompressed
// bytes, so the helper throws "incomplete download". Its fallback then calls
// arrayBuffer() on a body that was already consumed, which is the
// "body stream already read" error users see. This version treats
// Content-Length as a hint and never re-reads the body.

/**
 * @param {string} url
 * @param {string} mimeType
 * @param {{
 *   onProgress?: (p: {received: number, total: number}) => void,
 *   totalHint?: number,
 *   fetchImpl?: typeof fetch,
 * }} [opts]
 * @returns {Promise<{url: string, bytes: number}>}
 */
export async function fetchToBlobURL(url, mimeType, opts = {}) {
  const { onProgress, totalHint = 0, fetchImpl = globalThis.fetch } = opts;
  const resp = await fetchImpl(url);
  if (!resp.ok) throw new Error(`HTTP ${resp.status} while fetching ${url}`);

  const total = expectedTotal(resp.headers, totalHint);
  let data;
  if (resp.body && typeof resp.body.getReader === 'function') {
    const reader = resp.body.getReader();
    const chunks = [];
    let received = 0;
    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      chunks.push(value);
      received += value.length;
      // Never report more than 100%: the hint can be stale after a core upgrade.
      onProgress?.({ received, total: Math.max(total, received) });
    }
    data = new Uint8Array(received);
    let pos = 0;
    for (const c of chunks) {
      data.set(c, pos);
      pos += c.length;
    }
  } else {
    data = new Uint8Array(await resp.arrayBuffer());
    onProgress?.({ received: data.length, total: data.length });
  }
  const blobURL = URL.createObjectURL(new Blob([data], { type: mimeType }));
  return { url: blobURL, bytes: data.length };
}

/**
 * Content-Length is the on-the-wire size. With a Content-Encoding it is the
 * compressed size and useless as a progress total, so the caller's hint wins.
 * Exported for tests.
 * @param {Headers} headers
 * @param {number} [totalHint]
 */
export function expectedTotal(headers, totalHint = 0) {
  const encoding = headers.get('content-encoding');
  const length = Number(headers.get('content-length')) || 0;
  if (encoding && encoding !== 'identity') return totalHint || 0;
  return length || totalHint || 0;
}
