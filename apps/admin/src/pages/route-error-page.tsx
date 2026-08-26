import { AlertTriangle, Home, RefreshCw, SearchX } from 'lucide-react'
import { useEffect } from 'react'
import { Link, isRouteErrorResponse, useRouteError } from 'react-router'

import { Button, buttonVariants } from '@/components/ui/button'
import { cn } from '@/lib/utils'

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
    document.title = `${title} · News Control Room`
    return () => {
      document.title = previousTitle
    }
  }, [title])
}

function RecoveryContent({ description, headingId, retry = false, status, title }: RecoveryContentProps) {
  const Icon = retry ? AlertTriangle : SearchX

  return (
    <div className="w-full max-w-lg text-center">
      <div className="mx-auto flex size-14 items-center justify-center rounded-2xl border border-border bg-muted/50 text-muted-foreground">
        <Icon aria-hidden="true" className="size-6" />
      </div>
      <p className="mt-6 text-xs font-semibold tracking-[0.18em] text-muted-foreground uppercase">Error {status}</p>
      <h1 id={headingId} className="mt-2 text-2xl font-semibold tracking-tight sm:text-3xl">{title}</h1>
      <p className="mx-auto mt-3 max-w-md text-sm leading-6 text-muted-foreground">{description}</p>
      <div className="mt-7 flex flex-col-reverse justify-center gap-2 sm:flex-row">
        {retry ? (
          <Button variant="outline" size="lg" onClick={() => window.location.reload()}>
            <RefreshCw aria-hidden="true" />
            Try again
          </Button>
        ) : null}
        <Link className={cn(buttonVariants({ size: 'lg' }), 'no-underline')} to="/" replace>
          <Home aria-hidden="true" />
          Back to dashboard
        </Link>
      </div>
    </div>
  )
}

export function NotFoundPage() {
  usePageTitle('Page not found')

  return (
    <main className="flex min-h-svh items-center justify-center bg-background px-6 py-16 text-foreground">
      <section className="w-full max-w-xl rounded-3xl border border-border bg-card px-6 py-12 shadow-sm sm:px-10" aria-labelledby="not-found-title">
        <RecoveryContent
          headingId="not-found-title"
          status="404"
          title="Page not found"
          description="The address may be incorrect, or this page may have moved. Return to the dashboard to continue working."
        />
      </section>
    </main>
  )
}

export function RouteErrorPage() {
  const error = useRouteError()
  const notFound = isRouteErrorResponse(error) && error.status === 404
  const title = notFound ? 'Page not found' : 'Something went wrong'
  usePageTitle(title)

  return (
    <main className="flex min-h-svh items-center justify-center bg-background px-6 py-16 text-foreground">
      <section className="w-full max-w-xl rounded-3xl border border-border bg-card px-6 py-12 shadow-sm sm:px-10" aria-labelledby="route-error-title">
        <RecoveryContent
          headingId="route-error-title"
          status={notFound ? '404' : '500'}
          title={title}
          description={notFound
            ? 'The address may be incorrect, or this page may have moved. Return to the dashboard to continue working.'
            : 'The control room could not open this page. Your data is safe; reload the page or return to the dashboard.'}
          retry={!notFound}
        />
      </section>
    </main>
  )
}
