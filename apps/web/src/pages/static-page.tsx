export function StaticPage({ title, body }: { title: string; body: string }) {
  return (
    <article className="max-w-[65ch]">
      <h1 className="font-headline text-[length:var(--text-h1)] font-bold">{title}</h1>
      <p className="mt-6 leading-[1.75]">{body}</p>
    </article>
  )
}
