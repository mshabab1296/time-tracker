import { useTags } from './useTags'
import { IconButton } from '../../components/ui/IconButton'
import { EmptyState } from '../../components/ui/EmptyState'
import { RenameModal } from '../projects/RenameModal'

export function TagsPage() {
  const {
    tags,
    isAdmin,
    createResource,
    tagName,
    setTagName,
    openRename,
    deleteResource,
    renameTarget,
    renameName,
    setRenameName,
    setRenameTarget,
    submitRename,
  } = useTags()

  return (
    <>
      <section className="card page-card">
        <div className="section-heading">
          <div>
            <h2>Tags</h2>
            <p>Optional labels available to everyone in this workspace.</p>
          </div>
          <span className="count-chip">{tags.length}</span>
        </div>
        {isAdmin && (
          <form className="create-row" onSubmit={(event) => void createResource(event)}>
            <input
              placeholder="New tag name"
              value={tagName}
              maxLength={100}
              required
              onChange={(event) => setTagName(event.target.value)}
            />
            <button aria-label="Add tag">Add</button>
          </form>
        )}
        <div className="tag-grid">
          {tags.length ? (
            tags.map((tag) => (
              <div className="tag-card" key={tag.id}>
                <strong>{tag.name}</strong>
                {isAdmin && (
                  <span>
                    <IconButton
                      label={`Rename ${tag.name}`}
                      icon="✎"
                      onClick={() => openRename(tag)}
                    />
                    <IconButton
                      label={`Delete ${tag.name}`}
                      icon="⌫"
                      danger
                      onClick={() => void deleteResource(tag)}
                    />
                  </span>
                )}
              </div>
            ))
          ) : (
            <EmptyState
              title="No tags yet"
              text="Tags make it easier to organize future entries."
              compact
            />
          )}
        </div>
      </section>
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
