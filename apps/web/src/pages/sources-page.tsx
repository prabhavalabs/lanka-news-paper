import { createClient } from '@snap/api-client'
import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router'

const client = createClient()
const typeLabels: Record<string, string> = {
  private_media: 'පෞද්ගලික මාධ්‍ය',
  state_owned: 'රාජ්‍ය මාධ්‍ය',
  government: 'රජය',
  independent: 'ස්වාධීන',
  international: 'ජාත්‍යන්තර',
  other: 'වෙනත්',
}

export function SourcesPage() {
  const sources = useQuery({ queryKey: ['public-sources'], queryFn: () => client.sources() })
  return (
    <section>
      <h1 className="font-headline text-[length:var(--text-h1)] font-bold">මූලාශ්‍ර</h1>
      <ul className="mt-6">
        {sources.data?.items.map((source) => (
          <li key={source.id} className="border-b border-rule py-3">
            <Link to={`/sources/${source.id}`}>{source.name}</Link>
            <p className="text-[0.8125rem] text-muted-foreground">{typeLabels[source.type] ?? source.type}</p>
            {source.description ? <p className="mt-1 text-[0.8125rem]">{source.description}</p> : null}
          </li>
        ))}
      </ul>
    </section>
  )
}
