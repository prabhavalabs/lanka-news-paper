import { createClient } from '@snap/api-client'
import { useQuery } from '@tanstack/react-query'

const client = createClient()

export function BriefPage() {
  const brief = useQuery({ queryKey: ['brief'], queryFn: () => client.brief() })
  return (
    <article className="max-w-[65ch]">
      <h1 className="font-headline text-[length:var(--text-h1)] font-bold">{brief.data?.title_si ?? 'උදෑසන සංග්‍රහය'}</h1>
      {brief.data?.model ? (
        <p className="mt-2 border border-rule bg-tint px-3 py-2 text-[0.75rem]">
          ස්වයංක්‍රීය සාරාංශය · {brief.data.model}
        </p>
      ) : null}
      <pre className="font-body mt-6 whitespace-pre-wrap text-[length:var(--text-body)] leading-[1.75]">
        {brief.data?.body_si}
      </pre>
    </article>
  )
}
