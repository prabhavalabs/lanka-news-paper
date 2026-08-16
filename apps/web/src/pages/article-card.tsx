import type { PublicArticle } from '@snap/api-client'
import { Link } from 'react-router'

export const typeLabels: Record<string, string> = {
  private_media: 'පෞද්ගලික මාධ්‍ය',
  state_owned: 'රාජ්‍ය මාධ්‍ය',
  government: 'නිල නිවේදනය',
  independent: 'ස්වාධීන',
  international: 'ජාත්‍යන්තර',
  other: 'වෙනත්',
}

export function ArticleCard({ item, lead = false }: { item: PublicArticle; lead?: boolean }) {
  return (
    <article className="border-b border-rule py-5">
      {item.category ? <p className="text-[0.8125rem] text-muted-foreground">{item.category.name_si}</p> : null}
      {lead ? (
        <h1 className="font-headline mt-2 text-[length:var(--text-h1)] leading-[1.25] font-bold">
          <Link to={`/a/${item.id}`}>{item.headline}</Link>
        </h1>
      ) : (
        <h2 className="font-headline mt-1 text-[length:var(--text-h3)] leading-[1.35] font-bold">
          <Link to={`/a/${item.id}`}>{item.headline}</Link>
        </h2>
      )}
      <p className="mt-2 text-[0.8125rem]">
        මූලාශ්‍රය: {item.source.name} · {typeLabels[item.source.type] ?? item.source.type}
        {item.event_id ? ' · බහු-මූලාශ්‍ර සිදුවීම' : ' · තනි මූලාශ්‍ර වාර්තාව'}
      </p>
      <p className="mt-1 text-[0.8125rem] text-muted-foreground">
        පළ කළේ {new Date(item.published_at).toLocaleString('si-LK', { timeZone: 'Asia/Colombo' })}
        {' · '}
        ලැබුණේ {new Date(item.received_at).toLocaleString('si-LK', { timeZone: 'Asia/Colombo' })}
        {item.event_id ? (
          <>
            {' · '}
            <Link to={`/e/${item.event_id}`}>තවත් වාර්තා</Link>
          </>
        ) : null}
      </p>
      {item.editorial_note ? (
        <p className="mt-2 border border-rule bg-tint px-3 py-2 text-[0.8125rem]">වේදිකා සටහන: {item.editorial_note}</p>
      ) : null}
      <p className="mt-2">
        <a href={item.original_url} rel="noopener" className="underline">
          මුල් ලිපිය කියවන්න ↗ <span className="sr-only">(බාහිර සබැඳිය)</span>
        </a>
      </p>
    </article>
  )
}
