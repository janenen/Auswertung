// Minimal hash router: #/path?query

interface Route {
  path: string
  query: Record<string, string>
}

function parse(): Route {
  const hash = location.hash.slice(1) || '/'
  const [path, qs = ''] = hash.split('?')
  return { path, query: Object.fromEntries(new URLSearchParams(qs)) }
}

export const route = $state<Route>(parse())

window.addEventListener('hashchange', () => {
  const next = parse()
  route.path = next.path
  route.query = next.query
})

export function navigate(path: string, query?: Record<string, string | number | boolean | undefined>) {
  let target = path
  if (query) {
    const params = new URLSearchParams()
    for (const [k, v] of Object.entries(query)) {
      if (v !== undefined && v !== '') params.set(k, String(v))
    }
    const qs = params.toString()
    if (qs) target += '?' + qs
  }
  location.hash = target
}

export function href(path: string, query?: Record<string, string | number | boolean | undefined>) {
  let target = '#' + path
  if (query) {
    const qs = new URLSearchParams(
      Object.entries(query).filter(([, v]) => v !== undefined && v !== '') as [string, string][],
    ).toString()
    if (qs) target += '?' + qs
  }
  return target
}

/** Matches a pattern like /competitions/:id against a path. */
export function matchPath(pattern: string, path: string): Record<string, string> | null {
  const p = pattern.split('/')
  const s = path.split('/')
  if (p.length !== s.length) return null
  const params: Record<string, string> = {}
  for (let i = 0; i < p.length; i++) {
    if (p[i].startsWith(':')) params[p[i].slice(1)] = decodeURIComponent(s[i])
    else if (p[i] !== s[i]) return null
  }
  return params
}
