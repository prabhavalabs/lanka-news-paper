import { useEffect } from 'react'
import { Link, isRouteErrorResponse, useRouteError } from 'react-router'

import { Asterism } from '@/components/newsprint/ornament'

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
    document.title = `${title} · තුලාව`
    return () => {
      document.title = previousTitle
    }
  }, [title])
}

/** A printer's notice: double-framed, centred, with a plain way home. */
function RecoveryContent({ description, headingId, retry = false, status, title }: RecoveryContentProps) {
  return (
    <div className="mx-auto w-full max-w-xl border-[3px] border-double border-ink p-1 text-center">
      <div className="border border-ink px-5 py-10">
        <p className="np-meta">
          දැන්වීමයි · <span className="np-latin">{status}</span>
        </p>
        <Asterism className="mt-3 text-[1.25rem] text-ink-secondary" />
        <h1 id={headingId} className="np-headline mt-3 text-[2.25rem] leading-[1.35] md:text-[2.75rem]">
          {title}
        </h1>
        <p className="np-body mx-auto mt-3 max-w-md text-ink-secondary">{description}</p>
        <div className="mt-6 flex flex-col-reverse justify-center gap-3 sm:flex-row">
          {retry ? (
            <button type="button" className="np-btn np-btn-ghost" onClick={() => window.location.reload()}>
              නැවත උත්සාහ කරන්න
            </button>
          ) : null}
          <Link className="np-btn" to="/" replace>
            මුල් පිටුවට යන්න
          </Link>
        </div>
      </div>
    </div>
  )
}

export function NotFoundPage() {
  usePageTitle('පිටුව හමු නොවීය')

  return (
    <section className="py-10 md:py-16" aria-labelledby="not-found-title">
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
    <main className="flex min-h-svh items-center justify-center bg-paper px-4 py-16 text-ink">
      <section className="w-full" aria-labelledby="route-error-title">
        <RecoveryContent
          headingId="route-error-title"
          status={notFound ? '404' : '500'}
          title={title}
          description={
            notFound
              ? 'ඔබ ඇතුළත් කළ ලිපිනය වැරදි විය හැකිය, නැතහොත් මෙම පිටුව වෙනත් තැනකට ගෙන ගොස් ඇත.'
              : 'මෙම පිටුව දැන් විවෘත කළ නොහැක. නැවත උත්සාහ කරන්න, නැතහොත් මුල් පිටුවට යන්න.'
          }
          retry={!notFound}
        />
      </section>
    </main>
  )
}
