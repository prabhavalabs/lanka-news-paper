import { useQuery } from '@tanstack/react-query'
import { createClient } from '@snap/api-client'

const client = createClient()

export function FrontPage() {
  const news = useQuery({
    queryKey: ['news'],
    queryFn: () => client.listNews(),
  })

  return (
    <section className="grid grid-cols-1 gap-8 lg:grid-cols-12">
      <article className="border-b border-rule pb-8 lg:col-span-8 lg:border-e lg:border-b-0 lg:pe-8">
        <p className="text-[0.8125rem] text-[color:var(--ink-tertiary)]">නවතම</p>
        <h1 className="font-headline mt-2 text-[length:var(--text-h1)] leading-[1.25] font-bold">
          අදට තවම පුවත් නැත
        </h1>
        <p className="mt-3 text-[0.8125rem]">මූලාශ්‍රය: —</p>
        <p className="mt-4 max-w-[65ch] text-[color:var(--ink-secondary)]">
          අනුමත ආහාර ප්‍රවාහ සම්බන්ධ වූ පසු මෙහි මූලාශ්‍ර ශීර්ෂපාඨ පෙනෙනු ඇත. සම්පූර්ණ ලිපි මෙහි නැවත පළ නොවේ.
        </p>
      </article>
      <aside className="lg:col-span-4">
        <h2 className="border-t-2 border-ink pt-3 text-[length:var(--text-h2)] font-bold">දෙවන තීරුව</h2>
        {news.data?.items.length ? (
          <ol className="mt-4">
            {news.data.items.map((item) => (
              <li key={item.id} className="border-b border-rule py-3">
                <a href={item.original_url} className="font-headline text-[length:var(--text-h3)]">
                  {item.headline}
                </a>
                <p className="mt-1 text-[0.8125rem]">මූලාශ්‍රය: {item.source.name}</p>
              </li>
            ))}
          </ol>
        ) : (
          <p className="mt-4 border-b border-rule py-3 text-[color:var(--ink-secondary)]">
            බලාපොරොත්තුවෙන්.
          </p>
        )}
      </aside>
    </section>
  )
}
