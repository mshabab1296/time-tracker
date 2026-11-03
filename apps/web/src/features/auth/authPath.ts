export type AuthPage = 'login' | 'register' | 'forgot-password'

export function authPath(page: AuthPage, invitationToken?: string) {
  if (!invitationToken) return `/${page}`
  const parameters = new URLSearchParams({ token: invitationToken, page })
  return `/invite?${parameters}`
}
