import { useEffect, useState, type FormEvent } from 'react'
import { api } from '../../api/client'
import type { Organization, User } from '../../types/domain'
import { useWorkspace } from '../dashboard/WorkspaceContext'

export function useSettings() {
  const {
    user,
    setUser,
    organization,
    organizationID,
    isAdmin,
    loadOrganizations,
    selectOrganization,
    setMessage,
  } = useWorkspace()
  const [profileName, setProfileName] = useState(user.name)

  const [organizationName, setOrganizationName] = useState('')

  const [organizationTimezone, setOrganizationTimezone] = useState('')

  const [newOrganizationName, setNewOrganizationName] = useState('')

  async function saveProfile(event: FormEvent) {
    event.preventDefault()
    const result = await api<User>('/profile', {
      method: 'PATCH',
      body: JSON.stringify({ name: profileName }),
    })
    if (result.error) setMessage(result.error.message, 'error')
    else {
      setUser(result.data)
      setMessage('Profile saved.')
    }
  }

  async function saveOrganization(event: FormEvent) {
    event.preventDefault()
    const result = await api<Organization>(`/organizations/${organizationID}`, {
      method: 'PATCH',
      body: JSON.stringify({ name: organizationName, timezone: organizationTimezone }),
    })
    if (result.error) setMessage(result.error.message, 'error')
    else {
      await loadOrganizations()
      setMessage('Organization saved.')
    }
  }

  async function createOrganization(event: FormEvent) {
    event.preventDefault()
    const result = await api<Organization>('/organizations', {
      method: 'POST',
      body: JSON.stringify({
        name: newOrganizationName,
        timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
      }),
    })
    if (result.error) setMessage(result.error.message, 'error')
    else {
      setNewOrganizationName('')
      await loadOrganizations()
      selectOrganization(result.data.id)
      setMessage('Organization created.')
    }
  }
  useEffect(() => {
    if (organization) {
      setOrganizationName(organization.name)
      setOrganizationTimezone(organization.timezone)
    }
  }, [organization])
  return {
    saveProfile,
    profileName,
    setProfileName,
    user,
    isAdmin,
    saveOrganization,
    organizationName,
    setOrganizationName,
    organizationTimezone,
    setOrganizationTimezone,
    createOrganization,
    newOrganizationName,
    setNewOrganizationName,
  }
}
