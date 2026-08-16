import { Link, Outlet } from 'react-router'

export function ShellLayout() {
  return (
    <div className="min-h-screen bg-paper text-ink">
      <header className="flex h-12 items-center justify-between border-b border-rule px-6">
        <p className="text-sm font-medium">SNAP newsroom</p>
        <nav className="flex gap-4 text-sm">
          <Link to="/">Sources</Link>
          <Link to="/login">Sign in</Link>
        </nav>
      </header>
      <main className="mx-auto max-w-5xl px-6 py-8">
        <Outlet />
      </main>
    </div>
  )
}
