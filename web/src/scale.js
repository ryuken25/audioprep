/**
 * Fit an input frame inside a preset box without upscaling.
 *
 * @param {number} inW   coded width
 * @param {number} inH   coded height
 * @param {number} rotation  rotation metadata in degrees (0, 90, 180, 270, -90 ...)
 * @param {number} maxW  preset box width (landscape orientation)
 * @param {number} maxH  preset box height
 * @returns {{w:number, h:number, scaled:boolean}} target size in display orientation.
 *   `scaled` is false when the input already fits and has even dimensions,
 *   in which case the caller omits the scale filter entirely.
 */
export function fitDimensions(inW, inH, rotation, maxW, maxH) {
  let w = Math.max(1, Math.floor(Number(inW) || 0));
  let h = Math.max(1, Math.floor(Number(inH) || 0));

  const rot = ((Math.round(Number(rotation) || 0) % 360) + 360) % 360;
  if (rot === 90 || rot === 270) [w, h] = [h, w];

  let boxW = Number(maxW) || w;
  let boxH = Number(maxH) || h;
  // Swap the box when the input orientation and the box orientation disagree.
  const inPortrait = h > w;
  const boxPortrait = boxH > boxW;
  if (inPortrait !== boxPortrait && boxW !== boxH && w !== h) [boxW, boxH] = [boxH, boxW];

  const ratio = Math.min(boxW / w, boxH / h, 1); // never upscale
  let outW = Math.floor(w * ratio);
  let outH = Math.floor(h * ratio);
  outW = Math.max(2, outW - (outW % 2));
  outH = Math.max(2, outH - (outH % 2));

  const scaled = outW !== w || outH !== h;
  return { w: outW, h: outH, scaled };
}

/**
 * Output frame for a still-picture encode (an audio input + a cover picture).
 * The picture is padded to this exact box, so the cover tile in the UI can use the
 * same aspect ratio as the finished video. Null when the preset has no video box.
 *
 * @param {number|null} maxW preset box width
 * @param {number|null} maxH preset box height
 * @returns {{w:number, h:number}|null} even-sized box
 */
export function coverBox(maxW, maxH, coverW = 0, coverH = 0) {
  let w = Math.floor(Number(maxW));
  let h = Math.floor(Number(maxH));
  if (!Number.isFinite(w) || !Number.isFinite(h) || w < 2 || h < 2) return null;
  // Turn the box to match the picture, so portrait art gives a portrait video
  // instead of one with huge black pillars. A square picture keeps the
  // preset's own orientation. This mirrors Request.CoverBox in the Go app;
  // the two must agree or the same input gives two different outputs.
  const cw = Math.floor(Number(coverW)) || 0;
  const ch = Math.floor(Number(coverH)) || 0;
  if (cw > 0 && ch > 0 && cw !== ch && (ch > cw) !== (h > w)) {
    [w, h] = [h, w];
  }
  return { w: w - (w % 2), h: h - (h % 2) };
}

/**
 * Size a picture takes inside a box, aspect ratio kept. Mirrors ffmpeg's
 * `scale=W:H:force_original_aspect_ratio=decrease`, which also scales small
 * pictures up to fill the box (unlike fitDimensions, which never upscales).
 */
export function fitInsideBox(inW, inH, boxW, boxH) {
  const w = Math.max(1, Math.floor(Number(inW) || 0));
  const h = Math.max(1, Math.floor(Number(inH) || 0));
  const bw = Math.max(1, Math.floor(Number(boxW) || 0));
  const bh = Math.max(1, Math.floor(Number(boxH) || 0));
  const ratio = Math.min(bw / w, bh / h);
  return { w: Math.max(1, Math.round(w * ratio)), h: Math.max(1, Math.round(h * ratio)) };
}
