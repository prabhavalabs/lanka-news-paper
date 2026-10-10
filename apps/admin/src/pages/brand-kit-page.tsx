import { CopyIcon, DownloadIcon } from 'lucide-react'
import { toast } from 'sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'

const kit = '/brand/kit'

const downloads = [
  { label: 'Facebook Page kit', file: 'thulawa-facebook-page-kit.zip', note: 'Profile picture, cover photo, logos, link preview, Page text and guidelines' },
  { label: 'Complete brand kit', file: 'thulawa-brand-kit-complete.zip', note: 'Every logo, wordmark, layer, icon, font and the rebuild scripts' },
  { label: 'Brand guidelines', file: 'thulawa-brand-guidelines.pdf', note: '9-page PDF: idea, logo, colour, type, Facebook, icons, rules' },
  { label: 'Facebook cover (recommended)', file: 'facebook-cover-2-mobile-1640x924.jpg', note: 'Mobile and desktop, centre kept clear' },
  { label: 'Facebook cover, desktop', file: 'facebook-cover-3-desktop-1640x924.jpg', note: 'Desktop only' },
  { label: 'Facebook cover, classic', file: 'facebook-cover-1-classic-1640x924.jpg', note: 'Original layout' },
  { label: 'Facebook profile picture', file: 'facebook-profile-720.jpg', note: '720 × 720' },
  { label: 'Seal (logo)', file: 'thulawa-seal-2000.png', note: '2000 × 2000, transparent' },
  { label: 'Horizontal logo, cream', file: 'lockup-horizontal-cream.png', note: '1600 × 600' },
  { label: 'Horizontal logo, navy', file: 'lockup-horizontal-navy.png', note: '1600 × 600' },
  { label: 'Link preview image', file: 'og-image-1200x630.jpg', note: '1200 × 630' },
]

const colours = [
  { name: 'Navy', hex: '#13233A', token: '--brand-navy', use: 'Seal ring, headlines on light, primary' },
  { name: 'Deep navy', hex: '#0B1626', token: '--brand-navy-deep', use: 'Dark backgrounds and covers' },
  { name: 'Gold', hex: '#C9A24A', token: '--brand-gold', use: 'Name, rules and accents' },
  { name: 'Light gold', hex: '#E9D08C', token: '--brand-gold-light', use: 'Tagline on navy' },
  { name: 'Dark gold', hex: '#94702A', token: '--brand-gold-dark', use: 'Tagline and accents on cream' },
  { name: 'Cream', hex: '#F5ECD8', token: '--brand-cream', use: 'Light backgrounds, seal centre' },
]

const typefaces = [
  { role: 'Name තුලාව', font: 'Stick No Bills', weight: 'Bold 700', note: 'Logo name only, supplied as artwork.' },
  { role: 'Tagline', font: 'Gemunu Libre', weight: 'Bold 700', note: 'Draws ෝ without its hal; use the tagline artwork, or check ෝ words when typing.' },
  { role: 'English', font: 'Bodoni Moda', weight: 'Bold 700, +12% tracking', note: 'THULAWA NEWS and English headings.' },
  { role: 'Reading', font: 'Noto Serif Sinhala / Noto Sans Sinhala', weight: '400–700', note: 'Article text, UI and long copy.' },
]

const facebook = [
  { label: 'Page name', value: 'Thulawa News - තුලාව' },
  { label: 'Username', value: '@thulawanews' },
  { label: 'Category', value: 'News & media website' },
  { label: 'Bio', value: 'අසමතුලිත ලෝකයකට සමතුලිත පුවත්. එකම පුවත, මූලාශ්‍ර කිහිපයකින්.' },
  { label: 'Bio (English)', value: 'Balanced news for an unbalanced world. One story, many sources, every slant shown.' },
  { label: 'Website', value: 'https://thulawanews.com' },
]

async function copy(value: string) {
  await navigator.clipboard.writeText(value)
  toast.success('Copied')
}

export function BrandKitPage() {
  return (
    <section className="flex flex-col gap-6">
      <section className="flex flex-col gap-6 overflow-hidden rounded-xl bg-brand-navy-deep p-6 text-brand-cream md:flex-row md:items-center md:p-8">
        <img src="/brand/thulawa-seal-512.png" alt="Thulawa seal" className="size-36 shrink-0 md:size-44" />
        <div className="flex flex-col gap-4">
          <img src="/brand/name-gold.svg" alt="තුලාව" className="h-16 w-auto self-start md:h-20" />
          <img src="/brand/tagline-gold-light.svg" alt="අසමතුලිත ලෝකයකට සමතුලිත පුවත්" className="h-6 w-auto self-start" />
          <p className="max-w-xl text-sm text-brand-cream/75">
            Logos, colours, fonts and Facebook Page details for Thulawa News. Use the supplied artwork; never retype the name or tagline.
          </p>
        </div>
      </section>

      <Card>
        <CardHeader>
          <CardTitle>Downloads</CardTitle>
          <CardDescription>Master files live in design/brand-kit in the repository.</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
          {downloads.map((item) => (
            <div key={item.file} className="flex items-center justify-between gap-3 rounded-lg border p-3">
              <div className="min-w-0">
                <p className="truncate text-sm font-medium">{item.label}</p>
                <p className="truncate text-xs text-muted-foreground">{item.note}</p>
              </div>
              <Button variant="outline" size="sm" nativeButton={false} render={<a href={`${kit}/${item.file}`} download />}>
                <DownloadIcon /> Download
              </Button>
            </div>
          ))}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Colour</CardTitle>
          <CardDescription>Navy and gold carry the brand. Reading text stays ink on paper; the bias colours are unchanged.</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-6">
          {colours.map((colour) => (
            <button
              key={colour.hex}
              type="button"
              onClick={() => copy(colour.hex)}
              className="flex flex-col overflow-hidden rounded-lg border text-left transition-colors hover:border-foreground/40"
              aria-label={`Copy ${colour.name} ${colour.hex}`}
            >
              <span className="h-20 border-b" style={{ background: colour.hex }} />
              <span className="flex flex-col gap-0.5 p-3 text-xs">
                <span className="flex items-center justify-between text-sm font-medium">
                  {colour.name} <CopyIcon className="size-3.5 text-muted-foreground" />
                </span>
                <span className="font-mono">{colour.hex}</span>
                <span className="font-mono text-muted-foreground">{colour.token}</span>
                <span className="text-muted-foreground">{colour.use}</span>
              </span>
            </button>
          ))}
        </CardContent>
      </Card>

      <div className="grid gap-6 xl:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Typography</CardTitle>
            <CardDescription>All fonts are free under the SIL Open Font License.</CardDescription>
          </CardHeader>
          <CardContent className="flex flex-col divide-y">
            {typefaces.map((type) => (
              <div key={type.role} className="flex flex-col gap-1 py-3 first:pt-0 last:pb-0">
                <div className="flex flex-wrap items-center gap-2">
                  <p className="text-sm font-medium">{type.font}</p>
                  <Badge variant="secondary">{type.role}</Badge>
                  <span className="text-xs text-muted-foreground">{type.weight}</span>
                </div>
                <p className="text-xs text-muted-foreground">{type.note}</p>
              </div>
            ))}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Facebook Page details</CardTitle>
            <CardDescription>Click a value to copy it. Check the username is free when creating the Page.</CardDescription>
          </CardHeader>
          <CardContent className="flex flex-col divide-y">
            {facebook.map((row) => (
              <button
                key={row.label}
                type="button"
                onClick={() => copy(row.value)}
                className="grid grid-cols-[8rem_1fr_auto] items-center gap-3 py-2.5 text-left first:pt-0 last:pb-0"
              >
                <span className="text-xs text-muted-foreground">{row.label}</span>
                <span className="text-sm">{row.value}</span>
                <CopyIcon className="size-3.5 text-muted-foreground" />
              </button>
            ))}
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Usage rules</CardTitle>
        </CardHeader>
        <CardContent className="grid gap-6 text-sm md:grid-cols-2">
          <ul className="flex list-disc flex-col gap-1.5 ps-5">
            <li>Use the supplied seal, wordmarks and lockups.</li>
            <li>Place the seal on navy, deep navy or cream, with clear space equal to the ring width.</li>
            <li>Below 96 px, use the app icon (illustration only) instead of the full seal.</li>
            <li>Keep the tagline exactly: අසමතුලිත ලෝකයකට සමතුලිත පුවත්</li>
          </ul>
          <ul className="flex list-disc flex-col gap-1.5 ps-5 text-muted-foreground">
            <li>Don't retype the name or tagline in another font.</li>
            <li>Don't recolour, stretch, rotate or add effects to the seal.</li>
            <li>Don't put gold text on cream, or navy on deep navy.</li>
            <li>Don't use the old names ලංකා පුවත් or Lanka News Paper.</li>
          </ul>
        </CardContent>
      </Card>
    </section>
  )
}
