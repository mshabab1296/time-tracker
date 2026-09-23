export function IconButton({
  label,
  icon,
  danger = false,
  onClick,
}: {
  label: string
  icon: string
  danger?: boolean
  onClick: () => void
}) {
  return (
    <button
      type="button"
      aria-label={label}
      className={`icon-button ${danger ? 'danger' : ''}`}
      onClick={onClick}
      title={label}
    >
      {icon}
    </button>
  )
}
