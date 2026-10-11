const publicOrigins: Record<string, string> = {
  'admin.thulawanews.com': 'https://thulawanews.com',
  'admin.lankanewspaper.prabhavalabs.com': 'https://lankanewspaper.prabhavalabs.com',
}

/** Origin of the public reader site for the admin host this page runs on (5174 → 5173 locally). */
export function publicOrigin() {
  const { hostname, protocol, port } = window.location
  return publicOrigins[hostname] ?? `${protocol}//${hostname}${port === '5174' ? ':5173' : port ? `:${port}` : ''}`
}

export function publicURL(path: string) {
  return `${publicOrigin()}${path}`
}
