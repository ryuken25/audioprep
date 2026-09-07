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
