import { Link } from 'react-router'

import { Asterism } from './ornament'

const links = [
  { to: '/about', label: 'අප ගැන සහ පුවත්පත සකසන ආකාරය' },
  { to: '/editorial', label: 'කතුවැකි' },
  { to: '/sources', label: 'මූලාශ්‍ර මාධ්‍ය' },
  { to: '/brief', label: 'උදෑසන සංග්‍රහය' },
  { to: '/corrections', label: 'නිවැරදි කිරීම්' },
  { to: '/privacy', label: 'රහස්‍යතාව' },
  { to: '/contact', label: 'සම්බන්ධ වන්න' },
]

/** Colophon: how the paper is made, and where every report comes from. */
export function SiteFooter() {
  return (
    <footer className="mx-auto mt-12 max-w-[1280px] px-4 pb-10 md:px-12">
      <div className="np-rule-double" />
      <div className="grid gap-6 py-6 md:grid-cols-[minmax(0,2fr)_minmax(0,1fr)] md:gap-10">
        <section aria-labelledby="colophon-title">
          <h2 id="colophon-title" className="np-headline text-[1.25rem] leading-[1.5]">
            මෙම පුවත්පත සකස් වන ආකාරය
          </h2>
          <p className="np-body mt-2 max-w-[62ch] text-[1rem] text-ink-secondary">
            තුලාව පුවත් ලියන ආයතනයක් නොවේ. ශ්‍රී ලංකාවේ මාධ්‍ය ආයතන පළ කළ වාර්තා අපි එක් තැනකට ගෙන එමු; එකම පුවත
            වාර්තා කළ සියලු මාධ්‍ය එකට පෙන්වමු; සෑම පුවතකම මුල් ලිපියට සබැඳියක් තබමු. මුල් පිටුවේ ප්‍රධාන පුවත් තෝරන්නේ
            ප්‍රධාන කතුවරයා ය; කතුවරයා තෝරා නැති ඉඩ නවතම වාර්තා අනුව ස්වයංක්‍රීයව පිරේ. කතුවැකිය කතුවරයාගේ මතය මිස පුවත්
            වාර්තාවක් නොවේ.
          </p>
          <p className="mt-1">
            <Link to="/about" className="np-link font-headline inline-flex min-h-11 items-center font-bold">
              වැඩි විස්තර →
            </Link>
          </p>
        </section>
        <nav aria-label="පාදක සබැඳි" className="md:border-s md:border-rule md:ps-8">
          <ul className="grid grid-cols-1 gap-x-6 sm:grid-cols-2 md:grid-cols-1">
            {links.map((link) => (
              <li key={link.to}>
                <Link to={link.to} className="np-link font-headline inline-flex min-h-11 items-center text-[1rem] font-bold">
                  {link.label}
                </Link>
              </li>
            ))}
          </ul>
        </nav>
      </div>
      <div className="np-rule" />
      <p className="np-meta mt-3 flex flex-wrap items-center justify-center gap-x-3 text-center text-[0.875rem]">
        <span>තුලාව — අසමතුලිත ලෝකයකට සමතුලිත පුවත්</span>
        <Asterism className="text-ink-tertiary" />
        <span lang="en" className="np-latin">
          Thulawa News · thulawanews.com · Colombo
        </span>
      </p>
    </footer>
  )
}
