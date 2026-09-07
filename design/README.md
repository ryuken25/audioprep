# design/

Everything needed to hand Kenshi AudioPrep to Claude Design (or any
designer) for a visual pass, without them having to read the code.

| File | What it is |
|---|---|
| `prompt.md` | The brief. Product, every screen and state, every button and what it does, constraints, deliverables. Paste this into Claude Design. |
| `persona.md` | Who the owner is, what they make, how they talk, what their images look like. Scraped from x.com/kenshi2k_. |
| `brand.json` | Colour tokens (with the image each came from), type direction, mood words, things to avoid. |
| `assets/avatar.jpg` | Profile picture, 321x321. Used for the app icon and the site header. |
| `assets/banner.jpg` | X banner, 1500x500. Source of the gold and sea-grey tokens. |
| `assets/covers/` | Six cover-video thumbnails as posted. Show the black-and-white VRChat-photo style the covers use. |
| `assets/photos/` | Fan art and a photo from the timeline. |
| `current-screens/` | `01` to `03`: v0.2 dark, including the Custom state the owner disliked. `05` to `08`: v0.3 after the quick fix (dark, plus the light preset area). `09` is a full-page light capture whose dark card fills are a screenshot-tool artifact, kept only so nobody rediscovers it. |

How to use it with Claude Design: attach the whole folder, paste `prompt.md`
as the message, and ask for the deliverables listed at its end. The prompt
already says what must not change (the encoding pipeline, the preset
values, the single-page structure).
