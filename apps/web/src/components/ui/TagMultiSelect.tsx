import { useState } from 'react'
import type { Tag } from '../../types/domain'
import { MultiSelect } from './MultiSelect'

export function TagMultiSelect({
  tags,
  selectedIds,
  onToggle,
  showRecentByDefault = false,
  disabled = false,
}: {
  tags: Tag[]
  selectedIds: string[]
  onToggle: (id: string) => void
  showRecentByDefault?: boolean
  disabled?: boolean
}) {
  const [query, setQuery] = useState('')
  const [open, setOpen] = useState(false)
  const selected = selectedIds
    .map((id) => tags.find((tag) => tag.id === id))
    .filter((tag): tag is Tag => Boolean(tag))
    .map((tag) => ({ id: tag.id, label: tag.name }))
  const available = tags.filter((tag) => !selectedIds.includes(tag.id))
  const options = query.trim()
    ? available.filter((tag) =>
        tag.name.toLocaleLowerCase().includes(query.trim().toLocaleLowerCase()),
      )
    : showRecentByDefault
      ? [...available]
          .sort(
            (a, b) =>
              (b.createdAt ?? '').localeCompare(a.createdAt ?? '') || a.name.localeCompare(b.name),
          )
          .slice(0, 5)
      : []

  return (
    <MultiSelect
      label="Tags"
      selected={selected}
      options={options.map((tag) => ({ id: tag.id, label: tag.name }))}
      query={query}
      open={open}
      onQueryChange={setQuery}
      onOpenChange={setOpen}
      onSelect={onToggle}
      onRemove={onToggle}
      placeholder={selected.length ? 'Add tag…' : 'Search tags…'}
      emptyMessage="No matching tags"
      disabled={disabled}
    />
  )
}
