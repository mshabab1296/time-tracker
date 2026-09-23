import { useState, type FormEvent } from 'react'
import { api } from '../../api/client'
import type { RenameTarget, Tag } from '../../types/domain'
import { useWorkspace } from '../dashboard/WorkspaceContext'

export function useTags() {
  const { organizationID, isAdmin, tags, loadResources, setMessage } = useWorkspace()
  const [tagName, setTagName] = useState('')
  const [renameTarget, setRenameTarget] = useState<RenameTarget | null>(null)
  const [renameName, setRenameName] = useState('')

  async function createResource(event: FormEvent) {
    event.preventDefault()
    const result = await api<Tag>(`/organizations/${organizationID}/tags`, {
      method: 'POST',
      body: JSON.stringify({ name: tagName }),
    })
    if (result.error) setMessage(result.error.message, 'error')
    else {
      setTagName('')
      await loadResources()
      setMessage('Tag created.')
    }
  }

  function openRename(resource: Tag) {
    setRenameTarget({ kind: 'tags', resource })
    setRenameName(resource.name)
  }

  async function submitRename(event: FormEvent) {
    event.preventDefault()
    if (!renameTarget || !renameName.trim()) return
    const result = await api<Tag>(
      `/organizations/${organizationID}/tags/${renameTarget.resource.id}`,
      { method: 'PATCH', body: JSON.stringify({ name: renameName.trim() }) },
    )
    if (result.error) setMessage(result.error.message, 'error')
    else {
      setRenameTarget(null)
      await loadResources()
      setMessage('Tag renamed.')
    }
  }

  async function deleteResource(resource: Tag) {
    if (!window.confirm(`Delete ${resource.name}?`)) return
    const result = await api<{ deleted: boolean }>(
      `/organizations/${organizationID}/tags/${resource.id}`,
      { method: 'DELETE' },
    )
    if (result.error) setMessage(result.error.message, 'error')
    else {
      await loadResources()
      setMessage('Tag deleted.')
    }
  }

  return {
    tags,
    isAdmin,
    tagName,
    setTagName,
    createResource,
    openRename,
    deleteResource,
    renameTarget,
    renameName,
    setRenameName,
    setRenameTarget,
    submitRename,
  }
}
