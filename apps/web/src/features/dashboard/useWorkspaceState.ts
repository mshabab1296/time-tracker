import { useCallback, useEffect, useMemo, useState } from 'react'
import { api } from '../../api/client'
import type { Notice } from '../../types/notice'
import type {
  Invitation,
  Member,
  Organization,
  Project,
  SentInvitation,
  Tag,
  User,
} from '../../types/domain'

export function useWorkspaceState(initialUser: User, onLogout: () => void) {
  const [user, setUser] = useState(initialUser)
  const [organizations, setOrganizations] = useState<Organization[]>([])
  const [organizationID, setOrganizationID] = useState(
    localStorage.getItem('timetracker.organizationId') ?? '',
  )
  const [projects, setProjects] = useState<Project[]>([])
  const [tags, setTags] = useState<Tag[]>([])
  const [members, setMembers] = useState<Member[]>([])
  const [incoming, setIncoming] = useState<Invitation[]>([])
  const [sent, setSent] = useState<SentInvitation[]>([])
  const [notice, setNotice] = useState<Notice | null>(null)
  const [entriesRevision, setEntriesRevision] = useState(0)

  const organization = useMemo(
    () => organizations.find((item) => item.id === organizationID),
    [organizations, organizationID],
  )
  const isAdmin = organization?.role === 'ADMIN'

  const setMessage = useCallback((text: string, type: 'success' | 'error' = 'success') => {
    setNotice({ text, type })
  }, [])
  const dismissNotice = useCallback(() => setNotice(null), [])
  const selectOrganization = useCallback((id: string) => {
    setOrganizationID(id)
    setProjects([])
    setTags([])
    setMembers([])
    setSent([])
    localStorage.setItem('timetracker.organizationId', id)
  }, [])
  const loadOrganizations = useCallback(async () => {
    const result = await api<Organization[]>('/organizations')
    if (result.error) return setMessage(result.error.message, 'error')
    setOrganizations(result.data)
    if (!organizationID && result.data[0]) selectOrganization(result.data[0].id)
  }, [organizationID, selectOrganization, setMessage])
  const loadResources = useCallback(async () => {
    if (!organizationID) return
    const [projectResult, tagResult] = await Promise.all([
      api<Project[]>(`/organizations/${organizationID}/projects`),
      api<Tag[]>(`/organizations/${organizationID}/tags`),
    ])
    if (!projectResult.error) setProjects(projectResult.data)
    if (!tagResult.error) setTags(tagResult.data)
    if (organization?.role === 'ADMIN') {
      const [memberResult, invitationResult] = await Promise.all([
        api<Member[]>(`/organizations/${organizationID}/members`),
        api<SentInvitation[]>(`/organizations/${organizationID}/invitations`),
      ])
      setMembers(memberResult.error ? [] : memberResult.data)
      setSent(invitationResult.error ? [] : invitationResult.data)
    } else {
      setMembers([])
      setSent([])
    }
  }, [organizationID, organization?.role])
  const loadIncoming = useCallback(async () => {
    const result = await api<Invitation[]>('/invitations')
    if (!result.error) setIncoming(result.data)
  }, [])
  const notifyEntriesChanged = useCallback(() => setEntriesRevision((value) => value + 1), [])
  const logout = useCallback(async () => {
    await api<unknown>('/auth/logout', { method: 'POST' })
    onLogout()
  }, [onLogout])

  useEffect(() => {
    void loadOrganizations()
  }, [loadOrganizations])
  useEffect(() => {
    void loadResources()
  }, [loadResources])
  useEffect(() => {
    void loadIncoming()
  }, [loadIncoming])
  useEffect(() => {
    if (!notice) return
    const id = window.setTimeout(dismissNotice, 4500)
    return () => window.clearTimeout(id)
  }, [notice, dismissNotice])

  return {
    user,
    setUser,
    organizations,
    organizationID,
    organization,
    isAdmin,
    projects,
    tags,
    members,
    incoming,
    sent,
    notice,
    setMessage,
    dismissNotice,
    selectOrganization,
    loadOrganizations,
    loadResources,
    loadIncoming,
    entriesRevision,
    notifyEntriesChanged,
    logout,
  }
}
