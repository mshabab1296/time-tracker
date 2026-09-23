import { useProjects } from './useProjects'
import { SingleSelect } from '../../components/ui/SingleSelect'
import { IconButton } from '../../components/ui/IconButton'
import { EmptyState } from '../../components/ui/EmptyState'
import { RenameModal } from './RenameModal'

export function ProjectsPage() {
  const {
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
  } = useProjects()

  return (
    <>
      <div className="split-layout">
        <section className="card">
          <div className="section-heading">
            <div>
              <h2>Projects</h2>
              <p>Projects are required when recording time.</p>
            </div>
            <span className="count-chip">{projects.length}</span>
          </div>
          {isAdmin && (
            <form className="create-row" onSubmit={(event) => void createResource(event)}>
              <input
                placeholder="New project name"
                value={projectName}
                maxLength={100}
                required
                onChange={(event) => setProjectName(event.target.value)}
              />
              <button aria-label="Add project">Add</button>
            </form>
          )}
          <div className="resource-table">
            {projects.length ? (
              projects.map((project) => (
                <div className="resource-row" key={project.id}>
                  <span>
                    <strong>{project.name}</strong>
                    {isAdmin && (
                      <small>{project.assignedMemberIds?.length ?? 0} Members assigned</small>
                    )}
                  </span>
                  {isAdmin && (
                    <span className="row-actions">
                      <IconButton
                        label={`Rename ${project.name}`}
                        icon="✎"
                        onClick={() => openRename(project)}
                      />
                      <IconButton
                        label={`Delete ${project.name}`}
                        icon="⌫"
                        danger
                        onClick={() => void deleteResource(project)}
                      />
                    </span>
                  )}
                </div>
              ))
            ) : (
              <EmptyState
                title="No projects yet"
                text="Create your first project to begin tracking."
                compact
              />
            )}
          </div>
        </section>
        {isAdmin && (
          <section className="card">
            <h2>Assignments</h2>
            <p className="muted">Only assigned Members can choose a project.</p>
            <label>
              Project
              <SingleSelect
                label="Project"
                value={assignmentProjectID}
                onValueChange={setAssignmentProjectID}
                options={projects.map((project) => ({ value: project.id, label: project.name }))}
              />
            </label>
            <div className="assignment-list">
              {assigned.length ? (
                assigned.map((member) => (
                  <div className="assignment-row" key={member.id}>
                    <span>
                      <strong>{member.name}</strong>
                      <small>{member.email}</small>
                    </span>
                    <button
                      className="link"
                      onClick={() => void changeAssignment(member.id, 'unassign')}
                    >
                      Remove
                    </button>
                  </div>
                ))
              ) : (
                <p className="muted">No Members assigned.</p>
              )}
            </div>
            {assignable.length > 0 && (
              <div className="create-row">
                <SingleSelect
                  label="Member to assign"
                  value={assignmentMemberID}
                  onValueChange={setAssignmentMemberID}
                  options={[
                    { value: '', label: 'Select a Member' },
                    ...assignable.map((member) => ({ value: member.id, label: member.name })),
                  ]}
                />
                <button
                  onClick={() =>
                    assignmentMemberID && void changeAssignment(assignmentMemberID, 'assign')
                  }
                  disabled={!assignmentMemberID}
                >
                  Assign
                </button>
              </div>
            )}
          </section>
        )}
      </div>
      {renameTarget && (
        <RenameModal
          target={renameTarget}
          name={renameName}
          onNameChange={setRenameName}
          onClose={() => setRenameTarget(null)}
          onSubmit={(event) => void submitRename(event)}
        />
      )}
    </>
  )
}
