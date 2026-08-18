import { createClient } from '@snap/api-client'
import { useQuery } from '@tanstack/react-query'
import type { CSSProperties } from 'react'
import { useEffect } from 'react'
import { Outlet, useNavigate } from 'react-router'

import { AppSidebar } from '@/components/app-sidebar'
import { SiteHeader } from '@/components/site-header'
import { SidebarInset, SidebarProvider } from '@/components/ui/sidebar'

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
    return <p className="p-8 text-sm text-muted-foreground">Loading newsroom…</p>
  }

  if (me.isError) {
    return null
  }

  return (
    <SidebarProvider
      style={{
        '--sidebar-width': 'calc(var(--spacing) * 68)',
        '--header-height': 'calc(var(--spacing) * 12)',
      } as CSSProperties}
    >
      <AppSidebar
        variant="inset"
        user={me.data}
        onLogout={async () => {
          try {
            await client.logout()
          } finally {
            navigate('/login')
          }
        }}
      />
      <SidebarInset>
        <SiteHeader />
        <main className="@container/main flex flex-1 flex-col p-4 lg:p-6">
          <Outlet />
        </main>
      </SidebarInset>
    </SidebarProvider>
  )
}
