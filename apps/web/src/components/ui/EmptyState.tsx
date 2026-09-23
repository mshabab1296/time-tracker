export function EmptyState({
  title,
  text,
  compact = false,
}: {
  title: string
  text: string
  compact?: boolean
}) {
  return (
    <div className={`empty-state ${compact ? 'compact' : ''}`}>
      <span>◌</span>
      <strong>{title}</strong>
      <p>{text}</p>
    </div>
  )
}
