// Kenshi AudioPrep UI strings, EN / ID / JP.
//
// The bulk of this table is the owner's own copy pass, taken verbatim from the
// approved design and not to be reworded. The exceptions, added later because
// the design had no key for them, are `background` and the fn.warn* entries
// other than warnDur: those are written in the same register but were not
// reviewed by the owner, so treat them as drafts.
// Every user-visible string in the app. Three languages, one table.
// The `fn` entries are formatters: they own their own word order and units, so
// callers must never build a sentence out of fragments.
//
// The table is the design deliverable (design/from-claude-design/strings.js.txt)
// copied verbatim. Do not paraphrase or retranslate; add keys instead.

export const STR = {
  en: {
    code: 'EN', subtitle: "So your covers don't get crushed on X / TikTok. Runs 100% in your browser.",
    desktop: 'Desktop app', background: 'Background', themeToggle: 'Toggle dark / light theme', langToggle: 'Language: English. Click for Bahasa Indonesia',
    dropTitle: 'Drop your cover here, or click to browse', dropHint: 'MP4, MOV, MKV, WebM, AVI, M4A, WAV or MP3. Nothing leaves your device.',
    input: 'Input', size: 'Size', duration: 'Duration', video: 'Video', videoBitrate: 'Video bitrate', audio: 'Audio', audioBitrate: 'Audio bitrate', statusRow: 'Status', reading: 'Reading…',
    rotated: 'rotated 90°', over: 'over 140 s', audioOnlyBadge: 'audio only', none: 'none',
    noVideo: 'No video stream. The output will be an MP4 with a picture, or a black frame. Pick Audio only (M4A) if you just want the audio.',
    picture: 'Picture', pictureOptional: 'Optional', choosePicture: 'Choose a picture', changePicture: 'Change picture', remove: 'Remove', blackFrame: 'black frame', staticPicture: 'static picture',
    pictureHint: 'JPG, PNG or WebP. Scaled to fit the preset box, never stretched. Shown for the whole track.',
    preset: 'Preset', output: 'Output', staticFrame: 'static frame',
    presets: { 'x-audio': '720p at 24 fps, 1.8 Mb/s cap, 320k AAC, 16 kHz lowpass. The one for vocal covers.', 'x-balanced': '1080p, 6 Mb/s cap, 256k AAC. Sharper picture, bigger file.', tiktok: '1080x1920 vertical, 8 Mb/s cap, 256k AAC. Phone-shaped.', 'audio-only': 'Drops the video. Normalized 256k AAC in an .m4a container.', custom: 'Every knob exposed. Starts from whatever you last selected.' },
    advanced: 'Advanced', loudness: 'Loudness', target: 'Target', truePeak: 'True peak', lra: 'Loudness range', audioGroup: 'Audio', bitrate: 'Bitrate', lowpass: 'Lowpass filter', cutoff: 'Cutoff', audioOnly: 'Audio only (drop video, output .m4a)', videoGroup: 'Video', crf: 'CRF', maxrate: 'Max rate', fpsCap: 'FPS cap', boxW: 'Box width', boxH: 'Box height', staticGroup: 'Static video', staticVideo: 'Single frame + audio (tiny file)', frameAt: 'Frame at',
    process: 'Process', processing: 'Processing…', cancel: 'Cancel', cancelling: 'Cancelling…',
    status: { loading: 'Warming up the FFmpeg core…', idle: 'Drop a file to begin. No rush.', ready: 'Ready to process.', cancelled: 'Cancelled. Nothing broke.', done: 'Done. Go upload it.', error: 'Encoding failed (ffmpeg exit code 1). See the log.' },
    stages: { load: 'Loading FFmpeg', read: 'Reading file', probe: 'Probing', measure: 'Measuring loudness (pass 1)', frame: 'Extracting still frame', encode: 'Encoding (pass 2)', check: 'Checking output' },
    result: 'Result', cmpInput: 'Input', cmpOutput: 'Output', overall: 'Overall bitrate', loudnessRow: 'Loudness', download: 'Download', another: 'Process another',
    log: 'Log', logHint: 'Every ffmpeg command and its output.', clear: 'Clear', logEmpty: "# log is empty. so is kenshi's sleep schedule.",
    coreErr: 'FFmpeg core failed to load.', coreLoading: 'FFmpeg core: loading…', coreReady: 'FFmpeg core: ready', coreReadyNote: 'single-threaded wasm',
    honest: 'X will still re-encode your video. This just hands it the cleanest possible input; it does not skip the compression.',
    fn: { eta: (m, s) => m ? `about ${m} min ${s} s left` : `about ${s} s left`, dur: (m, s) => m ? `${m} min ${s} s` : `${s} s`, summary: (d, I, TP, size) => `Done in ${d}. ${I} LUFS, ${TP} dBTP, ${size}.`, warnDur: (d, max) => `Duration is ${d} s. X allows up to ${max} s for most accounts.`, warnSize: (mb, max) => `File is ${mb} MB. X rejects uploads over ${max} MB.`, warnRes: (w, h, mw, mh) => `Resolution ${w}x${h} is over X's ${mw}x${mh} limit.`, warnFps: (fps, max) => `Frame rate ${fps} fps is above ${max}. X may reject it or drop frames.`, lines: (n) => n === 1 ? '1 line' : `${n} lines`, coverEmpty: (w, h) => `No picture yet. The video will be a black ${w}x${h} frame, which X and TikTok accept fine.`, coverSet: (name, w, h, ow, oh) => `${name} · ${ow}x${oh}, scaled to ${w}x${h}.` },
  },
  id: {
    code: 'ID', subtitle: 'Biar cover kamu nggak pecah di X / TikTok. Jalan 100% di browser.',
    desktop: 'Aplikasi desktop', background: 'Latar', themeToggle: 'Ganti tema gelap / terang', langToggle: 'Bahasa: Indonesia. Klik untuk 日本語',
    dropTitle: 'Drop cover kamu di sini, atau klik buat pilih file', dropHint: 'MP4, MOV, MKV, WebM, AVI, M4A, WAV atau MP3. Nggak ada yang keluar dari device kamu.',
    input: 'Input', size: 'Ukuran', duration: 'Durasi', video: 'Video', videoBitrate: 'Bitrate video', audio: 'Audio', audioBitrate: 'Bitrate audio', statusRow: 'Status', reading: 'Membaca…',
    rotated: 'diputar 90°', over: 'lebih dari 140 s', audioOnlyBadge: 'audio saja', none: 'nggak ada',
    noVideo: 'Nggak ada video stream. Outputnya jadi MP4 pakai gambar, atau frame hitam. Pilih Audio saja (M4A) kalau cuma mau audionya.',
    picture: 'Gambar', pictureOptional: 'Opsional', choosePicture: 'Pilih gambar', changePicture: 'Ganti gambar', remove: 'Hapus', blackFrame: 'frame hitam', staticPicture: 'gambar statis',
    pictureHint: 'JPG, PNG atau WebP. Di-scale biar pas kotak preset, nggak pernah ditarik. Tampil sepanjang lagu.',
    preset: 'Preset', output: 'Output', staticFrame: 'frame statis',
    presets: { 'x-audio': '720p 24 fps, cap 1.8 Mb/s, AAC 320k, lowpass 16 kHz. Paling pas buat vocal cover.', 'x-balanced': '1080p, cap 6 Mb/s, AAC 256k. Gambar lebih tajam, file lebih besar.', tiktok: '1080x1920 vertikal, cap 8 Mb/s, AAC 256k. Ukuran layar HP.', 'audio-only': 'Buang videonya. AAC 256k ternormalisasi dalam container .m4a.', custom: 'Semua knob kebuka. Mulai dari preset yang terakhir kamu pilih.' },
    advanced: 'Lanjutan', loudness: 'Loudness', target: 'Target', truePeak: 'True peak', lra: 'Loudness range', audioGroup: 'Audio', bitrate: 'Bitrate', lowpass: 'Filter lowpass', cutoff: 'Cutoff', audioOnly: 'Audio saja (buang video, output .m4a)', videoGroup: 'Video', crf: 'CRF', maxrate: 'Max rate', fpsCap: 'Batas FPS', boxW: 'Lebar box', boxH: 'Tinggi box', staticGroup: 'Video statis', staticVideo: 'Satu frame + audio (file kecil)', frameAt: 'Frame di detik',
    process: 'Proses', processing: 'Memproses…', cancel: 'Batal', cancelling: 'Membatalkan…',
    status: { loading: 'Manasin FFmpeg core dulu…', idle: 'Drop file dulu ya. Santai aja.', ready: 'Siap diproses.', cancelled: 'Dibatalin. Aman, nggak ada yang rusak.', done: 'Selesai. Gas upload.', error: 'Encoding gagal (ffmpeg exit code 1). Cek log.' },
    stages: { load: 'Memuat FFmpeg', read: 'Membaca file', probe: 'Mengecek file', measure: 'Mengukur loudness (pass 1)', frame: 'Mengambil satu frame', encode: 'Encoding (pass 2)', check: 'Mengecek hasil' },
    result: 'Hasil', cmpInput: 'Input', cmpOutput: 'Output', overall: 'Bitrate total', loudnessRow: 'Loudness', download: 'Download', another: 'Proses lagi',
    log: 'Log', logHint: 'Semua perintah ffmpeg dan outputnya.', clear: 'Bersihkan', logEmpty: '# log masih kosong. sama kayak jam tidur kenshi.',
    coreErr: 'FFmpeg core gagal dimuat.', coreLoading: 'FFmpeg core: memuat…', coreReady: 'FFmpeg core: siap', coreReadyNote: 'wasm single-thread',
    honest: 'X tetap bakal re-encode video kamu. Ini cuma ngasih input sebersih mungkin, bukan lewatin kompresinya.',
    fn: { eta: (m, s) => m ? `sekitar ${m} mnt ${s} s lagi` : `sekitar ${s} s lagi`, dur: (m, s) => m ? `${m} mnt ${s} s` : `${s} s`, summary: (d, I, TP, size) => `Selesai dalam ${d}. ${I} LUFS, ${TP} dBTP, ${size}.`, warnDur: (d, max) => `Durasinya ${d} s. X cuma kasih sampai ${max} s buat kebanyakan akun.`, warnSize: (mb, max) => `Filenya ${mb} MB. X nolak upload di atas ${max} MB.`, warnRes: (w, h, mw, mh) => `Resolusinya ${w}x${h}, lewat batas X yang ${mw}x${mh}.`, warnFps: (fps, max) => `Frame rate-nya ${fps} fps, di atas ${max}. X bisa nolak atau nurunin.`, lines: (n) => `${n} baris`, coverEmpty: (w, h) => `Belum ada gambar. Videonya jadi frame hitam ${w}x${h}, X dan TikTok terima kok.`, coverSet: (name, w, h, ow, oh) => `${name} · ${ow}x${oh}, di-scale ke ${w}x${h}.` },
  },
  jp: {
    code: 'JP', subtitle: '歌ってみたがXやTikTokで潰れないように。100%ブラウザの中で動きます。',
    desktop: 'デスクトップ版', background: '背景', themeToggle: 'ダーク / ライトを切り替え', langToggle: '言語: 日本語。クリックで English',
    dropTitle: 'ここに動画をドロップ、またはクリックして選ぶ', dropHint: 'MP4、MOV、MKV、WebM、AVI、M4A、WAV、MP3。端末の外には何も送りません。',
    input: '入力', size: 'サイズ', duration: '長さ', video: 'ビデオ', videoBitrate: 'ビデオビットレート', audio: 'オーディオ', audioBitrate: 'オーディオビットレート', statusRow: '状態', reading: '読み込み中…',
    rotated: '90° 回転', over: '140 s 超え', audioOnlyBadge: '音声のみ', none: 'なし',
    noVideo: '映像ストリームがありません。出力は画像つき、または黒い画面の MP4 になります。音声だけなら Audio only (M4A) を選んでね。',
    picture: '画像', pictureOptional: '任意', choosePicture: '画像を選ぶ', changePicture: '画像を変える', remove: '外す', blackFrame: '黒い画面', staticPicture: '静止画',
    pictureHint: 'JPG、PNG、WebP。プリセットの枠に収まるように縮小、引き伸ばしはしません。曲の間ずっと表示。',
    preset: 'プリセット', output: '出力', staticFrame: '静止フレーム',
    presets: { 'x-audio': '720p・24 fps、上限 1.8 Mb/s、AAC 320k、16 kHz ローパス。歌ってみたはこれ。', 'x-balanced': '1080p、上限 6 Mb/s、AAC 256k。画はきれい、ファイルは重め。', tiktok: '1080x1920 縦、上限 8 Mb/s、AAC 256k。スマホの形。', 'audio-only': '映像を捨てる。ノーマライズ済み AAC 256k を .m4a で。', custom: '全部いじれる。最後に選んだプリセットから始まる。' },
    advanced: '詳細設定', loudness: 'ラウドネス', target: 'ターゲット', truePeak: 'トゥルーピーク', lra: 'ラウドネスレンジ', audioGroup: 'オーディオ', bitrate: 'ビットレート', lowpass: 'ローパスフィルター', cutoff: 'カットオフ', audioOnly: '音声のみ（映像を捨てて .m4a 出力）', videoGroup: 'ビデオ', crf: 'CRF', maxrate: '最大レート', fpsCap: 'FPS 上限', boxW: 'ボックス幅', boxH: 'ボックス高さ', staticGroup: '静止映像', staticVideo: '1フレーム + 音声（極小ファイル）', frameAt: 'フレーム位置',
    process: '変換する', processing: '処理中…', cancel: 'キャンセル', cancelling: 'キャンセル中…',
    status: { loading: 'FFmpeg core を起こしてます…', idle: 'ファイルをドロップしてね。急がなくていいよ。', ready: '準備OK。', cancelled: 'キャンセルしました。何も壊れてないよ。', done: '完了。あとはアップするだけ。', error: 'エンコードに失敗しました（ffmpeg exit code 1）。ログを確認してください。' },
    stages: { load: 'FFmpeg を読み込み中', read: 'ファイルを読み込み中', probe: '解析中', measure: 'ラウドネス測定（パス1）', frame: '静止フレームを抽出中', encode: 'エンコード中（パス2）', check: '出力を確認中' },
    result: '結果', cmpInput: '入力', cmpOutput: '出力', overall: '全体ビットレート', loudnessRow: 'ラウドネス', download: 'ダウンロード', another: '別のファイルを処理',
    log: 'ログ', logHint: 'すべての ffmpeg コマンドとその出力。', clear: 'クリア', logEmpty: '# ログはまだ空。kenshi はたぶん寝てる。',
    coreErr: 'FFmpeg core の読み込みに失敗しました。', coreLoading: 'FFmpeg core: 読み込み中…', coreReady: 'FFmpeg core: 準備完了', coreReadyNote: 'シングルスレッド wasm',
    honest: 'X はそれでも動画を再エンコードします。これは一番きれいな入力を渡すだけで、圧縮を回避するものではありません。',
    fn: { eta: (m, s) => m ? `残り約 ${m} 分 ${s} 秒` : `残り約 ${s} 秒`, dur: (m, s) => m ? `${m} 分 ${s} 秒` : `${s} 秒`, summary: (d, I, TP, size) => `${d}で完了。${I} LUFS、${TP} dBTP、${size}。`, warnDur: (d, max) => `長さが ${d} s あります。X はほとんどのアカウントで ${max} s までです。`, warnSize: (mb, max) => `ファイルが ${mb} MB あります。X は ${max} MB を超えるとアップできません。`, warnRes: (w, h, mw, mh) => `解像度が ${w}x${h} で、X の上限 ${mw}x${mh} を超えています。`, warnFps: (fps, max) => `フレームレートが ${fps} fps で、${max} を超えています。X が弾くか間引くかもしれません。`, lines: (n) => `${n} 行`, coverEmpty: (w, h) => `画像はまだなし。映像は ${w}x${h} の黒い画面になります。X も TikTok もそれで通るよ。`, coverSet: (name, w, h, ow, oh) => `${name} · ${ow}x${oh} → ${w}x${h} に縮小。` },
  },
};

export const LANGS = ['en', 'id', 'jp'];
export const DEFAULT_LANG = 'en';

/** Value for <html lang>. Our key for Japanese is "jp"; the BCP 47 tag is "ja". */
const HTML_LANG = { en: 'en', id: 'id', jp: 'ja' };

const LS_KEY = 'kxc.lang';

/** Map a navigator.language value onto one of our three keys. */
export function langFromNavigator(tag) {
  const s = String(tag || '').toLowerCase();
  if (s.startsWith('id') || s.startsWith('in-')) return 'id'; // "in" is the legacy tag for Indonesian
  if (s.startsWith('ja') || s.startsWith('jp')) return 'jp';
  return DEFAULT_LANG;
}

let current = null;

function read() {
  try {
    const saved = localStorage.getItem(LS_KEY);
    if (LANGS.includes(saved)) return saved;
  } catch { /* private mode */ }
  if (typeof navigator !== 'undefined') return langFromNavigator(navigator.language);
  return DEFAULT_LANG;
}

/** Current language key. Resolved once from localStorage, then from the browser. */
export function getLang() {
  if (!current) current = read();
  return current;
}

/** Switch language, persist it and reflect it on <html lang>. Returns the applied key. */
export function setLang(lang) {
  current = LANGS.includes(lang) ? lang : DEFAULT_LANG;
  try { localStorage.setItem(LS_KEY, current); } catch { /* private mode */ }
  if (typeof document !== 'undefined') document.documentElement.lang = HTML_LANG[current];
  return current;
}

/** The language after this one in the EN -> ID -> JP cycle. */
export function nextLang(lang = getLang()) {
  return LANGS[(LANGS.indexOf(lang) + 1) % LANGS.length];
}

/**
 * Look up a string. `t()` returns the whole table for the current language, which is
 * what callers want when they need several keys or a `fn.*` formatter. `t('status.ready')`
 * resolves one dotted path and falls back to English, then to the key itself.
 */
export function t(key, lang = getLang()) {
  const table = STR[lang] || STR[DEFAULT_LANG];
  if (key === undefined) return table;
  const walk = (obj) => String(key).split('.').reduce((o, k) => (o == null ? undefined : o[k]), obj);
  const hit = walk(table);
  if (hit !== undefined) return hit;
  const fallback = walk(STR[DEFAULT_LANG]);
  return fallback !== undefined ? fallback : key;
}
