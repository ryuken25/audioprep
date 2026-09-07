import test from 'node:test';
import assert from 'node:assert/strict';
import { fetchToBlobURL, expectedTotal } from '../src/fetch-blob.js';

// A Response whose headers describe a gzipped transfer (small Content-Length)
// while the body stream yields the full decompressed bytes, which is exactly
// what a browser hands us from GitHub Pages.
function compressedLikeResponse(bytes, { contentLength, encoding = 'gzip', chunk = 1000 } = {}) {
  const body = new ReadableStream({
    start(controller) {
      for (let i = 0; i < bytes.length; i += chunk) controller.enqueue(bytes.subarray(i, i + chunk));
      controller.close();
    },
  });
  const headers = new Headers({ 'content-type': 'application/wasm' });
  if (contentLength != null) headers.set('content-length', String(contentLength));
  if (encoding) headers.set('content-encoding', encoding);
  return new Response(body, { status: 200, headers });
}

test('gzipped response with a smaller Content-Length than the body still succeeds', async () => {
  const bytes = new Uint8Array(32_000).map((_, i) => i & 255);
  const fetchImpl = async () => compressedLikeResponse(bytes, { contentLength: 10_000 });
  const seen = [];
  const out = await fetchToBlobURL('core.wasm', 'application/wasm', {
    fetchImpl,
    totalHint: 32_000,
    onProgress: (p) => seen.push(p),
  });
  assert.equal(out.bytes, 32_000);
  assert.ok(out.url.startsWith('blob:'));
  assert.equal(seen.at(-1).received, 32_000);
  assert.equal(seen.at(-1).total, 32_000, 'total comes from the hint, not the compressed Content-Length');
  assert.ok(seen.every((p) => p.received <= p.total), 'progress never exceeds 100%');
});

test('gzipped response with no hint still completes and caps total at received', async () => {
  const bytes = new Uint8Array(5_000);
  const fetchImpl = async () => compressedLikeResponse(bytes, { contentLength: 1_200 });
  const seen = [];
  const out = await fetchToBlobURL('core.wasm', 'application/wasm', { fetchImpl, onProgress: (p) => seen.push(p) });
  assert.equal(out.bytes, 5_000);
  assert.ok(seen.every((p) => p.received <= p.total));
});

test('expectedTotal trusts Content-Length only when the body is not encoded', () => {
  assert.equal(expectedTotal(new Headers({ 'content-length': '123' })), 123);
  assert.equal(expectedTotal(new Headers({ 'content-length': '123', 'content-encoding': 'gzip' }), 999), 999);
  assert.equal(expectedTotal(new Headers({ 'content-length': '123', 'content-encoding': 'br' })), 0);
  assert.equal(expectedTotal(new Headers({ 'content-length': '123', 'content-encoding': 'identity' })), 123);
  assert.equal(expectedTotal(new Headers(), 77), 77);
  assert.equal(expectedTotal(new Headers()), 0);
});

test('a response without a streaming body falls back to arrayBuffer once', async () => {
  const bytes = new Uint8Array([1, 2, 3, 4]);
  let reads = 0;
  const fetchImpl = async () => ({
    ok: true,
    status: 200,
    headers: new Headers(),
    body: null,
    arrayBuffer: async () => { reads++; return bytes.buffer; },
  });
  const out = await fetchToBlobURL('x.bin', 'application/octet-stream', { fetchImpl });
  assert.equal(out.bytes, 4);
  assert.equal(reads, 1);
});

test('non-2xx status rejects with the code in the message', async () => {
  const fetchImpl = async () => new Response('nope', { status: 404 });
  await assert.rejects(fetchToBlobURL('missing.wasm', 'application/wasm', { fetchImpl }), /HTTP 404/);
});
