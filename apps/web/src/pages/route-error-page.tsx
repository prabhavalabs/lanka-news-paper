import { Home, RefreshCw } from 'lucide-react'
import { useEffect } from 'react'
import { Link, isRouteErrorResponse, useRouteError } from 'react-router'

import { Button, buttonVariants } from '@/components/ui/button'
import { cn } from '@snap/ui/utils'

type RecoveryContentProps = {
  description: string
  headingId: string
  retry?: boolean
  status: string
  title: string
}

function usePageTitle(title: string) {
  useEffect(() => {
    const previousTitle = document.title
    document.title = `${title} · ලංකා පුවත්`
    return () => {
      document.title = previousTitle
    }
  }, [title])
}

function RecoveryContent({ description, headingId, retry = false, status, title }: RecoveryContentProps) {
  return (
    <div className="mx-auto w-full max-w-2xl text-center">
      <p className="font-sans text-xs font-semibold tracking-[0.2em] text-muted-foreground uppercase">Error {status}</p>
      <h1 id={headingId} className="font-headline mt-4 text-4xl leading-tight font-bold md:text-5xl">{title}</h1>
      <p className="mx-auto mt-5 max-w-xl text-base text-muted-foreground">{description}</p>
      <div className="mt-8 flex flex-col-reverse justify-center gap-3 sm:flex-row">
        {retry ? (
          <Button variant="outline" size="lg" onClick={() => window.location.reload()}>
            <RefreshCw aria-hidden="true" />
            නැවත උත්සාහ කරන්න
          </Button>
        ) : null}
        <Link className={cn(buttonVariants({ size: 'lg' }), 'no-underline')} to="/" replace>
          <Home aria-hidden="true" />
          මුල් පිටුවට යන්න
        </Link>
      </div>
    </div>
  )
}

export function NotFoundPage() {
  usePageTitle('පිටුව හමු නොවීය')

  return (
    <section className="border-y border-rule px-4 py-20 md:py-28" aria-labelledby="not-found-title">
      <RecoveryContent
        headingId="not-found-title"
        status="404"
        title="පිටුව හමු නොවීය"
        description="ඔබ ඇතුළත් කළ ලිපිනය වැරදි විය හැකිය, නැතහොත් මෙම පිටුව වෙනත් තැනකට ගෙන ගොස් ඇත."
      />
    </section>
  )
}

export function RouteErrorPage() {
  const error = useRouteError()
  const notFound = isRouteErrorResponse(error) && error.status === 404
  const title = notFound ? 'පිටුව හමු නොවීය' : 'යම් දෝෂයක් සිදු විය'
  usePageTitle(title)

  return (
    <main className="flex min-h-svh items-center justify-center bg-paper px-6 py-16 text-ink">
      <section className="w-full max-w-3xl border-y border-rule px-4 py-16" aria-labelledby="route-error-title">
        <RecoveryContent
          headingId="route-error-title"
          status={notFound ? '404' : '500'}
          title={title}
          description={notFound
            ? 'ඔබ ඇතුළත් කළ ලිපිනය වැරදි විය හැකිය, නැතහොත් මෙම පිටුව වෙනත් තැනකට ගෙන ගොස් ඇත.'
            : 'මෙම පිටුව දැන් විවෘත කළ නොහැක. නැවත උත්සාහ කරන්න, නැතහොත් මුල් පිටුවට යන්න.'}
          retry={!notFound}
        />
      </section>
    </main>
  )
}
