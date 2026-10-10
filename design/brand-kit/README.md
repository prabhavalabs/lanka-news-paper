# Thulawa News brand kit

Master copy of the Thulawa News (තුලාව) identity. The rules are in `thulawa-brand-guidelines.pdf` and `docs/brand-guidelines.md`.

| Folder | Contents |
|---|---|
| `logo/` | The seal (SVG, and PNG at 2000 / 1000 / 512 px) and horizontal lockups on cream, navy and transparent backgrounds |
| `wordmarks/` | Name, English name and tagline as outlined SVG and PNG in navy, gold, light gold, dark gold and cream |
| `layers/` | The seal split into ring, illustration and lettering (transparent 2000 px PNGs), plus the original AI illustration and its prompt |
| `facebook/` | Profile picture, cover photo, cover safe-area guide and `page-setup.md` with the Page name, bio and About text |
| `web/` | Favicons, app icons, maskable icon, link-preview image and the social post card masthead |
| `fonts/` | Stick No Bills, Gemunu Libre and Bodoni Moda (Bold), with their SIL Open Font Licenses |
| `source/` | Scripts, fonts and the AI illustration used to rebuild everything |

## Downloads

The ZIP downloads (`thulawa-facebook-page-kit.zip`, `thulawa-brand-kit-complete.zip`) and the guidelines PDF are served by the admin portal from `apps/admin/public/brand/kit/` and listed on its **Brand kit** page.

## Rebuilding

Requires Python 3 with fontTools and Pillow, HarfBuzz (`hb-shape`), `rsvg-convert` and Google Chrome.

```sh
cd design/brand-kit/source
python3 tools/brand_v2.py /tmp/thulawa-kit
python3 tools/guide_v2.py /tmp/thulawa-kit /tmp/thulawa-kit/thulawa-brand-guidelines.pdf
```

All lettering is converted to outlines with HarfBuzz shaping, so Sinhala renders exactly. Gemunu Libre draws ෝ without its hal; `outline.py` substitutes the font's correct `sinMatraAa.virama` glyph.

The editable Canva version is the design "Thulawa News · Brand kit (editable)" in the Canva folder "Thulawa News · Brand".
