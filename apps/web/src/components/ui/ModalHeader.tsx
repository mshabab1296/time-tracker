import { IconButton } from './IconButton'

export function ModalHeader({
  id,
  title,
  subtitle,
  onClose,
}: {
  id: string
  title: string
  subtitle: string
  onClose: () => void
}) {
  return (
    <header className="modal-header">
      <div>
        <h2 id={id}>{title}</h2>
        <p>{subtitle}</p>
      </div>
      <IconButton label="Close dialog" icon="×" onClick={onClose} />
    </header>
  )
}
