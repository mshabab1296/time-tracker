import { useEffect, useMemo, useState, type FormEvent } from 'react'
import { api } from '../../api/client'
import type { Project, RenameTarget } from '../../types/domain'
import { useWorkspace } from '../dashboard/WorkspaceContext'

export function useProjects() {
  const { organizationID, isAdmin, projects, members, loadResources, setMessage } = useWorkspace()
  const [renameTarget, setRenameTarget] = useState<RenameTarget | null>(null)

  const [renameName, setRenameName] = useState('')

  const [projectName, setProjectName] = useState('')

  const [assignmentProjectID, setAssignmentProjectID] = useState('')

  const [assignmentMemberID, setAssignmentMemberID] = useState('')

  const assignmentProject = useMemo(
    () => projects.find((item) => item.id === assignmentProjectID),
    [projects, assignmentProjectID],
  )

  async function createResource(event: FormEvent) {
    event.preventDefault()
    const result = await api<Project>(`/organizations/${organizationID}/projects`, {
      method: 'POST',
      body: JSON.stringify({ name: projectName }),
    })
    if (result.error) setMessage(result.error.message, 'error')
    else {
      setProjectName('')
      await loadResources()
      setMessage('Project created.')
    }
  }

  function openRename(resource: Project) {
    setRenameTarget({ kind: 'projects', resource })
    setRenameName(resource.name)
  }

  async function submitRename(event: FormEvent) {
    event.preventDefault()
    if (!renameTarget || !renameName.trim()) return
    const result = await api<Project>(
      `/organizations/${organizationID}/projects/${renameTarget.resource.id}`,
      { method: 'PATCH', body: JSON.stringify({ name: renameName.trim() }) },
    )
    if (result.error) setMessage(result.error.message, 'error')
    else {
      setRenameTarget(null)
      await loadResources()
      setMessage('Project renamed.')
    }
  }

  async function deleteResource(resource: Project) {
    if (!window.confirm(`Delete ${resource.name}?`)) return
    const result = await api<{ deleted: boolean }>(
      `/organizations/${organizationID}/projects/${resource.id}`,
      { method: 'DELETE' },
    )
    if (result.error) setMessage(result.error.message, 'error')
    else {
      await loadResources()
      setMessage('Project deleted.')
    }
  }

  async function changeAssignment(memberID: string, action: 'assign' | 'unassign') {
    if (!assignmentProjectID) return
    const result = await api<unknown>(
      `/organizations/${organizationID}/projects/${assignmentProjectID}/assignments/${memberID}`,
      { method: action === 'assign' ? 'POST' : 'DELETE' },
    )
    if (result.error) setMessage(result.error.message, 'error')
    else {
      await loadResources()
      setMessage(`Assignment ${action === 'assign' ? 'added' : 'removed'}.`)
    }
  }

  const assignmentIDs = assignmentProject?.assignedMemberIds ?? []

  const assigned = members.filter((member) => assignmentIDs.includes(member.id))

  const assignable = members.filter(
    (member) => member.role === 'MEMBER' && !assignmentIDs.includes(member.id),
  )
  useEffect(() => {
    if (!assignmentProjectID && projects[0]) setAssignmentProjectID(projects[0].id)
  }, [projects, assignmentProjectID])
  return {
    projects,
    isAdmin,
    createResource,
    projectName,
    setProjectName,
    openRename,
    deleteResource,
    assignmentProjectID,
    setAssignmentProjectID,
    assigned,
    changeAssignment,
    assignable,
    assignmentMemberID,
    setAssignmentMemberID,
    renameTarget,
    renameName,
    setRenameName,
    setRenameTarget,
    submitRename,
  }
}
