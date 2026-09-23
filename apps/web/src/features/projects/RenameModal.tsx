import type { FormEvent } from 'react'
import { ModalHeader } from '../../components/ui/ModalHeader'
import { ModalFooter } from '../../components/ui/ModalFooter'
import type { RenameTarget } from '../../types/domain'

export function RenameModal({
  target,
  name,
  onNameChange,
  onClose,
  onSubmit,
}: {
  target: RenameTarget
  name: string
  onNameChange: (name: string) => void
  onClose: () => void
  onSubmit: (event: FormEvent) => void
}) {
  return (
    <div className="modal-backdrop" role="presentation" onMouseDown={onClose}>
      <section
        className="modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="rename-title"
        onMouseDown={(event) => event.stopPropagation()}
      >
        <ModalHeader
          id="rename-title"
          title={`Rename ${target.kind.slice(0, -1)}`}
          subtitle={`Change the name of ${target.resource.name}.`}
          onClose={onClose}
        />
        <form className="stack" onSubmit={onSubmit}>
          <label>
            New name
            <input
              autoFocus
              value={name}
              maxLength={100}
              required
              onChange={(event) => onNameChange(event.target.value)}
            />
          </label>
          <ModalFooter>
            <button type="button" className="secondary" onClick={onClose}>
              Cancel
            </button>
            <button type="submit">Save name</button>
          </ModalFooter>
        </form>
      </section>
    </div>
  )
}
