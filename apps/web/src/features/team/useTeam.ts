import { useState, type FormEvent } from 'react'
import { api } from '../../api/client'
import type { Invitation, Member, SentInvitation } from '../../types/domain'
import { useWorkspace } from '../dashboard/WorkspaceContext'

export function useTeam() {
  const {
    organizationID,
    isAdmin,
    members,
    incoming,
    sent,
    loadResources,
    loadIncoming,
    loadOrganizations,
    setMessage,
  } = useWorkspace()
  const [inviteEmail, setInviteEmail] = useState('')

  async function invite(event: FormEvent) {
    event.preventDefault()
    const result = await api<unknown>(`/organizations/${organizationID}/invitations`, {
      method: 'POST',
      body: JSON.stringify({ email: inviteEmail }),
    })
    if (result.error) setMessage(result.error.message, 'error')
    else {
      setInviteEmail('')
      await loadResources()
      setMessage('Invitation sent.')
    }
  }

  async function invitationAction(invitation: SentInvitation, action: 'resend' | 'cancel') {
    const result = await api<unknown>(
      `/organizations/${organizationID}/invitations/${invitation.id}${action === 'resend' ? '/resend' : ''}`,
      { method: action === 'resend' ? 'POST' : 'DELETE' },
    )
    if (result.error) setMessage(result.error.message, 'error')
    else {
      await loadResources()
      setMessage(action === 'resend' ? 'Invitation resent.' : 'Invitation cancelled.')
    }
  }

  async function removeMember(member: Member) {
    if (!window.confirm(`Remove ${member.name}?`)) return
    const result = await api<unknown>(`/organizations/${organizationID}/members/${member.id}`, {
      method: 'DELETE',
    })
    if (result.error) setMessage(result.error.message, 'error')
    else {
      await loadResources()
      setMessage('Member removed.')
    }
  }

  async function decide(invitation: Invitation, decision: 'accept' | 'decline') {
    const result = await api<unknown>(`/invitations/${invitation.id}/${decision}`, {
      method: 'POST',
    })
    if (result.error) setMessage(result.error.message, 'error')
    else {
      await loadIncoming()
      await loadOrganizations()
      setMessage(`Invitation ${decision}ed.`)
    }
  }

  return {
    isAdmin,
    members,
    removeMember,
    incoming,
    decide,
    invite,
    inviteEmail,
    setInviteEmail,
    sent,
    invitationAction,
  }
}
