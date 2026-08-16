import { Link, Outlet, useNavigate } from 'react-router'
import { useQuery } from '@tanstack/react-query'
import { useEffect } from 'react'
import { createClient } from '@snap/api-client'

import { ThemeToggle } from '@/components/theme-toggle'

const client = createClient()

export function ShellLayout() {
  const navigate = useNavigate()
  const me = useQuery({ queryKey: ['me'], queryFn: () => client.me(), retry: false })

  useEffect(() => {
    if (me.isError) {
      navigate('/login')
    }
  }, [me.isError, navigate])

  if (me.isPending) {
    return <p className="p-8 text-sm text-muted-foreground">Loading…</p>
  }

  if (me.isError) {
    return null
  }

  return (
    <div className="min-h-screen bg-background text-foreground">
      <header className="flex h-12 items-center justify-between border-b border-border px-6">
        <p className="text-sm font-medium">SNAP newsroom</p>
        <nav className="flex items-center gap-4 text-sm">
          <Link to="/">Desk</Link>
          <Link to="/sources">Sources</Link>
          <Link to="/queue">Queue</Link>
          <Link to="/complaints">Complaints</Link>
          <Link to="/routing">AI & Routing</Link>
          <span className="text-muted-foreground">{me.data?.name} · {me.data?.role === 'administrator' ? 'Administrator' : me.data?.role}</span>
          <ThemeToggle />
          <button
            type="button"
            className="text-muted-foreground underline"
            onClick={async () => {
              await client.logout()
              navigate('/login')
            }}
          >
            Sign out
          </button>
        </nav>
      </header>
      <main className="mx-auto max-w-5xl px-6 py-8">
        <Outlet />
      </main>
    </div>
  )
}
