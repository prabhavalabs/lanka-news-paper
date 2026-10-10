# Brand guidelines: Thulawa News (තුලාව)

Source of truth for the Thulawa identity. The illustrated version is `design/brand-kit/thulawa-brand-guidelines.pdf`. All master files are in `design/brand-kit/`, and the admin portal's **Brand kit** page (`/brand`) offers the downloads.

## Name and tagline

| | Sinhala | English |
|---|---|---|
| Name | තුලාව | Thulawa News (logo lettering: THULAWA NEWS) |
| Tagline | අසමතුලිත ලෝකයකට සමතුලිත පුවත් | Balanced news for an unbalanced world |

තුලාව means *the scale*. The product puts many reports of the same story side by side, shows how each one leans, and leaves the judgement to the reader. The tagline echoes the name: තුලාව and සමතුලිත share a root.

Retired names: ලංකා පුවත්, Lanka News Paper, News Control Room. Do not use them in new copy.

## Logo

- **Seal** (primary): an AI-assisted engraved illustration (a fountain pen as the post of a balance, two pans, a sunburst and a Sri Lankan paddy wreath) inside a navy ring lettered THULAWA NEWS above and the tagline below.
- **Layers**: ring, illustration and lettering are separate files in `design/brand-kit/layers/`, so the lettering can change without redrawing the illustration.
- **Lockups**: seal plus name, English name and tagline, on cream, navy or transparent (`design/brand-kit/logo/`).
- **Icon**: illustration only on a navy tile, used for favicons and app icons.
- **Clear space**: at least the width of the navy ring around the seal.
- **Minimum size**: the full seal at 96 px / 25 mm; use the icon below that.

## Colour

| Name | Hex | Token | Use |
|---|---|---|---|
| Navy | `#13233A` | `--brand-navy` | Seal ring, headlines on light, brand surfaces |
| Deep navy | `#0B1626` | `--brand-navy-deep` | Dark backgrounds, covers |
| Gold | `#C9A24A` | `--brand-gold` | Name, rules, accents |
| Light gold | `#E9D08C` | `--brand-gold-light` | Tagline on navy |
| Dark gold | `#94702A` | `--brand-gold-dark` | Tagline and accents on cream or paper |
| Cream | `#F5ECD8` | `--brand-cream` | Light brand backgrounds, seal centre |

Pair gold or cream with navy, and navy or dark gold with cream. Avoid gold text on cream and navy on deep navy.

On the website, reading surfaces keep the newsprint palette from `docs/ui-guidelines.md` (paper `#F7F5F0`, ink `#141414`). Brand colours are used only for identity: masthead, logo, favicon, the social post card and brand pages. The bias colours stay as they are: left `#B96D70`, right `#587EAD`, breaking `#A52B26`.

## Typography

| Role | Font | Weight |
|---|---|---|
| Name තුලාව | Stick No Bills | Bold 700 |
| Tagline and Sinhala display | Gemunu Libre | Bold 700 |
| English (THULAWA NEWS, English headings) | Bodoni Moda | Bold 700, +12% letter-spacing |
| Reading and UI | Noto Serif Sinhala / Noto Sans Sinhala (web: Abhaya Libre headlines as before) | 400–700 |

All are free under the SIL Open Font License. The fonts are in `design/brand-kit/fonts/`.

The name, English name and tagline are always used as supplied artwork (outlined SVG/PNG), never retyped. **Gemunu Libre bug:** the font draws ෝ without its hal, so ලෝ looks like ලො. The supplied artwork is corrected; if you type Gemunu Libre text live, check every word with ෝ.

## Where the assets are used

| Surface | Files |
|---|---|
| Public website | `apps/web/public/brand/` (favicons, app icons, seal, wordmark SVGs, `og-image.jpg`) and `apps/web/public/manifest.webmanifest` |
| Admin portal | `apps/admin/public/brand/` (favicons, icon, seal) and `apps/admin/public/brand/kit/` (downloads) |
| Facebook post card | `services/api/internal/social/card/assets/brand/masthead.png`, inlined into the card |
| Facebook Page | `design/brand-kit/facebook/` and `page-setup.md` |
| Canva | Design "Thulawa News · Brand kit (editable)" in the folder "Thulawa News · Brand" |

## Rebuilding the artwork

The scripts in `design/brand-kit/source/` regenerate every file from the fonts and the AI illustration; see `design/brand-kit/README.md`. After rebuilding, copy the changed files into the web, admin and card locations above.
