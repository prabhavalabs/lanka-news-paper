import { createClient } from '@snap/api-client'
import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router'

import { OutletName } from '@/components/newsprint/outlet'
import { PageHead, useDocumentTitle } from '@/components/newsprint/page-head'
import { GalleyPlaceholder, Notice } from '@/components/newsprint/states'
import { editionDate, sinhalaLongDate } from '@/lib/format'

const client = createClient()

type BriefSection = { title: string; items: { text: string; source: string | null }[] }

/** The brief arrives as plain text: section names on their own lines, "• headline (Outlet)" items. */
function parseBrief(body: string): BriefSection[] {
  const sections: BriefSection[] = []
  for (const raw of body.split('\n')) {
    const line = raw.trim()
    if (!line) continue
    if (/^[•*-]\s*/.test(line)) {
      const text = line.replace(/^[•*-]\s*/, '')
      const match = /^(.*)\s+\(([^()]+)\)$/.exec(text)
      const item = match ? { text: match[1] ?? text, source: match[2] ?? null } : { text, source: null }
      if (!sections.length) sections.push({ title: '', items: [] })
      sections[sections.length - 1]?.items.push(item)
    } else {
      sections.push({ title: line, items: [] })
    }
  }
  return sections.filter((section) => section.items.length || section.title)
}

export function BriefPage() {
  const brief = useQuery({ queryKey: ['brief'], queryFn: () => client.brief() })
  const title = brief.data?.title_si ?? 'උදෑසන සංග්‍රහය'
  useDocumentTitle(title)
  const sections = brief.data ? parseBrief(brief.data.body_si) : []

  return (
    <section>
      <PageHead
        title={title}
        kicker={sinhalaLongDate(editionDate(brief.data?.date))}
        deck="පසුගිය පැය 24 තුළ මාධ්‍ය ආයතන පළ කළ ප්‍රධාන ශීර්ෂපාඨ, අංශ අනුව. සෑම ශීර්ෂපාඨයකටම යටින් ඇත්තේ එය පළ කළ මාධ්‍යයයි; ශීර්ෂපාඨය තෝරා මුල් ලිපිය සොයාගන්න."
      />
      {brief.isPending ? <GalleyPlaceholder className="mt-8" lines={10} /> : null}
      {brief.isError ? <Notice title="අද සංග්‍රහය තවම සකස් වී නැත" retry={() => brief.refetch()} className="mt-8" /> : null}
      {brief.data?.model ? (
        <p className="np-meta mt-3 text-center text-[0.8125rem]">ස්වයංක්‍රීයව එකතු කළ සංග්‍රහයකි</p>
      ) : null}
      <div className="np-columns mt-6 md:columns-2 lg:columns-3">
        {sections.map((section, index) => (
          <section key={`${section.title}-${index}`} className="mb-6 break-inside-avoid-column" aria-label={section.title || undefined}>
            {section.title ? <h2 className="np-section-head">{section.title}</h2> : null}
            <ul className="mt-1">
              {section.items.map((item, itemIndex) => (
                <li key={itemIndex} className="border-b border-rule py-2">
                  <p className="font-headline text-[1.0625rem] leading-[1.6] font-bold">
                    <Link to={`/search?q=${encodeURIComponent(item.text)}`} className="np-headline-link">
                      {item.text}
                    </Link>
                  </p>
                  {item.source ? (
                    <p className="np-meta text-[0.875rem]">
                      <OutletName name={item.source} className="font-normal text-ink-secondary" />
                    </p>
                  ) : null}
                </li>
              ))}
            </ul>
          </section>
        ))}
      </div>
    </section>
  )
}
