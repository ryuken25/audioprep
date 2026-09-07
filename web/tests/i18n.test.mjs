import { test } from 'node:test';
import assert from 'node:assert/strict';
import { STR, LANGS, DEFAULT_LANG, getLang, setLang, nextLang, langFromNavigator, t } from '../src/i18n.js';
import { PRESET_ORDER } from '../src/presets.js';
import { STAGES } from '../src/pipeline.js';

const FN_KEYS = ['eta', 'dur', 'summary', 'warnDur', 'lines', 'coverEmpty', 'coverSet'];

/** Every leaf path in a table, as dotted strings, with the leaf's typeof. */
function shape(obj, prefix = '', out = new Map()) {
  for (const [k, v] of Object.entries(obj)) {
    const path = prefix ? `${prefix}.${k}` : k;
    if (v && typeof v === 'object' && !Array.isArray(v)) shape(v, path, out);
    else out.set(path, typeof v);
  }
  return out;
}

test('three languages, no more and no less', () => {
  assert.deepEqual(LANGS, ['en', 'id', 'jp']);
  assert.deepEqual(Object.keys(STR).sort(), [...LANGS].sort());
  assert.equal(DEFAULT_LANG, 'en');
});

test('every key exists in all three languages with the same type', () => {
  const en = shape(STR.en);
  assert.ok(en.size > 90, `expected a full table, got ${en.size} keys`);
  for (const lang of LANGS) {
    const other = shape(STR[lang]);
    for (const [path, type] of en) {
      assert.ok(other.has(path), `${lang} is missing "${path}"`);
      assert.equal(other.get(path), type, `${lang}.${path} has the wrong type`);
    }
    for (const path of other.keys()) {
      assert.ok(en.has(path), `${lang} has an extra key "${path}" that en does not`);
    }
  }
});

test('no empty strings anywhere', () => {
  for (const lang of LANGS) {
    for (const [path, type] of shape(STR[lang])) {
      if (type !== 'string') continue;
      const value = path.split('.').reduce((o, k) => o[k], STR[lang]);
      assert.ok(value.trim().length > 0, `${lang}.${path} is empty`);
    }
  }
});

test('every fn entry is present and returns a non-empty string', () => {
  for (const lang of LANGS) {
    const fn = STR[lang].fn;
    assert.deepEqual(Object.keys(fn).sort(), [...FN_KEYS].sort(), `${lang}.fn has the wrong shape`);
    const calls = {
      eta: [2, 5],
      dur: [0, 42],
      summary: ['1 min 2 s', '-14.0', '-1.0', '3.2 MB'],
      warnDur: ['150.0', 140],
      lines: [12],
      coverEmpty: [1280, 720],
      coverSet: ['cover.jpg', 720, 720, 3000, 3000],
    };
    for (const key of FN_KEYS) {
      const out = fn[key](...calls[key]);
      assert.equal(typeof out, 'string', `${lang}.fn.${key} did not return a string`);
      assert.ok(out.trim().length > 0, `${lang}.fn.${key} returned an empty string`);
    }
    // eta and dur have a minutes branch and a seconds-only branch; both must be filled in.
    assert.notEqual(fn.eta(0, 9), fn.eta(3, 9));
    assert.notEqual(fn.dur(0, 9), fn.dur(3, 9));
    assert.ok(fn.lines(1).includes('1'));
    assert.ok(fn.coverEmpty(1080, 1920).includes('1080x1920'));
    assert.ok(fn.coverSet('a.png', 720, 720, 3000, 3000).includes('a.png'));
  }
});

test('preset descriptions cover exactly the preset list', () => {
  for (const lang of LANGS) {
    assert.deepEqual(Object.keys(STR[lang].presets).sort(), [...PRESET_ORDER].sort(), `${lang} preset descriptions`);
  }
});

test('every pipeline stage has a label in every language', () => {
  for (const lang of LANGS) {
    for (const key of Object.values(STAGES)) {
      assert.equal(typeof STR[lang].stages[key], 'string', `${lang} has no label for stage "${key}"`);
    }
  }
});

test('t() resolves dotted paths and falls back to English', () => {
  setLang('id');
  assert.equal(getLang(), 'id');
  assert.equal(t(), STR.id);
  assert.equal(t('status.ready'), 'Siap diproses.');
  assert.equal(t('presets.tiktok'), STR.id.presets.tiktok);
  assert.equal(t('status.ready', 'jp'), STR.jp.status.ready);
  assert.equal(t('nope.not.here'), 'nope.not.here', 'an unknown key returns itself');
  setLang('en');
});

test('language cycles EN -> ID -> JP -> EN', () => {
  assert.equal(nextLang('en'), 'id');
  assert.equal(nextLang('id'), 'jp');
  assert.equal(nextLang('jp'), 'en');
});

test('browser language maps onto the three codes', () => {
  assert.equal(langFromNavigator('id-ID'), 'id');
  assert.equal(langFromNavigator('in-ID'), 'id', 'legacy tag for Indonesian');
  assert.equal(langFromNavigator('ja-JP'), 'jp');
  assert.equal(langFromNavigator('en-GB'), 'en');
  assert.equal(langFromNavigator('de-DE'), 'en', 'anything else falls back to English');
  assert.equal(langFromNavigator(undefined), 'en');
});

test('setLang refuses an unknown code', () => {
  assert.equal(setLang('xx'), 'en');
  setLang('en');
});
