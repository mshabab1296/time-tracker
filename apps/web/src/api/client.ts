export type APIResponse<T> = { data: T; error: { code: string; message: string } | null }

function csrfToken() {
  return (
    document.cookie
      .split('; ')
      .find((value) => value.startsWith('timetracker_csrf='))
      ?.split('=')[1] ?? ''
  )
}

export async function api<T>(path: string, options: RequestInit = {}): Promise<APIResponse<T>> {
  const headers = new Headers(options.headers)
  if (options.body) headers.set('Content-Type', 'application/json')
  if (['POST', 'PATCH', 'PUT', 'DELETE'].includes(options.method ?? 'GET') && csrfToken())
    headers.set('X-CSRF-Token', csrfToken())
  return (
    await fetch(`/api/v1${path}`, { ...options, headers, credentials: 'include' })
  ).json() as Promise<APIResponse<T>>
}
