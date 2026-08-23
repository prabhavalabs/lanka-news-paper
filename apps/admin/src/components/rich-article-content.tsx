import rehypeRaw from 'rehype-raw'
import rehypeSanitize from 'rehype-sanitize'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'

import { cn } from '@/lib/utils'

export function RichArticleContent({ value, className }: { value: string; className?: string }) {
  if (!value.trim()) return <p className="text-sm text-muted-foreground">No readable content was persisted.</p>

  return (
    <div className={cn('min-w-0 text-sm leading-7 text-foreground/90', className)}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        rehypePlugins={[rehypeRaw, rehypeSanitize]}
        components={{
          h1: ({ children }) => <h1 className="mb-4 mt-6 break-words font-heading text-2xl font-semibold first:mt-0">{children}</h1>,
          h2: ({ children }) => <h2 className="mb-3 mt-6 break-words font-heading text-xl font-semibold first:mt-0">{children}</h2>,
          h3: ({ children }) => <h3 className="mb-2 mt-5 break-words font-heading text-lg font-semibold first:mt-0">{children}</h3>,
          h4: ({ children }) => <h4 className="mb-2 mt-4 break-words font-heading font-semibold first:mt-0">{children}</h4>,
          p: ({ children }) => <p className="mb-4 whitespace-pre-wrap break-words last:mb-0">{children}</p>,
          a: ({ children, href }) => <a className="break-words font-medium text-primary underline underline-offset-4" href={href} target="_blank" rel="noreferrer">{children}</a>,
          strong: ({ children }) => <strong className="font-semibold text-foreground">{children}</strong>,
          em: ({ children }) => <em>{children}</em>,
          ul: ({ children }) => <ul className="mb-4 list-disc space-y-1 pl-6">{children}</ul>,
          ol: ({ children }) => <ol className="mb-4 list-decimal space-y-1 pl-6">{children}</ol>,
          li: ({ children }) => <li className="break-words pl-1">{children}</li>,
          blockquote: ({ children }) => <blockquote className="my-4 border-l-2 border-primary/50 pl-4 italic text-muted-foreground">{children}</blockquote>,
          hr: () => <hr className="my-6 border-border" />,
          code: ({ children }) => <code className="rounded bg-muted px-1.5 py-0.5 font-mono text-[0.85em]">{children}</code>,
          pre: ({ children }) => <pre className="my-4 max-w-full overflow-x-auto rounded-lg border bg-muted/50 p-3 font-mono text-xs leading-5">{children}</pre>,
          table: ({ children }) => <div className="my-4 max-w-full overflow-x-auto rounded-lg border"><table className="w-full border-collapse text-left text-xs">{children}</table></div>,
          thead: ({ children }) => <thead className="bg-muted/70">{children}</thead>,
          th: ({ children }) => <th className="border-b px-3 py-2 font-semibold">{children}</th>,
          td: ({ children }) => <td className="border-b px-3 py-2 align-top last:border-b-0">{children}</td>,
          img: ({ alt, src, title }) => src ? <figure className="my-5 overflow-hidden rounded-xl border bg-muted/30"><img className="max-h-[28rem] w-full object-contain" src={src} alt={alt ?? ''} title={title} loading="lazy" decoding="async" />{alt ? <figcaption className="border-t px-3 py-2 text-center text-xs text-muted-foreground">{alt}</figcaption> : null}</figure> : null,
        }}
      >
        {value}
      </ReactMarkdown>
    </div>
  )
}
