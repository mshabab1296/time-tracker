import { useId, useState } from 'react'
import { SelectOptions } from './SelectOptions'

export type MultiSelectOption = { id: string; label: string; description?: string }

type Props = {
  label: string
  selected: MultiSelectOption[]
  options: MultiSelectOption[]
  query: string
  open: boolean
  onQueryChange: (query: string) => void
  onOpenChange: (open: boolean) => void
  onSelect: (id: string) => void
  onRemove: (id: string) => void
  placeholder: string
  emptyMessage: string
  disabled?: boolean
  closeOnSelect?: boolean
  showEmptyOnOpen?: boolean
}

export function MultiSelect({
  label,
  selected,
  options,
  query,
  open,
  onQueryChange,
  onOpenChange,
  onSelect,
  onRemove,
  placeholder,
  emptyMessage,
  disabled = false,
  closeOnSelect = false,
  showEmptyOnOpen = false,
}: Props) {
  const listId = useId()
  const [activeIndex, setActiveIndex] = useState(0)
  const focusedIndex = Math.max(0, Math.min(activeIndex, options.length - 1))
  const visible = open && (options.length > 0 || query.trim().length > 0 || showEmptyOnOpen)

  function choose(id: string) {
    onSelect(id)
    onQueryChange('')
    if (closeOnSelect) onOpenChange(false)
  }

  return (
    <div
      className="multi-select"
      onBlur={(event) => {
        if (!event.currentTarget.contains(event.relatedTarget)) onOpenChange(false)
      }}
    >
      <div className="multi-select-field">
        {selected.map((item) => (
          <button
            className="multi-select-chip"
            type="button"
            key={item.id}
            disabled={disabled}
            aria-label={`Remove ${item.label}`}
            title={`Remove ${item.label}`}
            onClick={() => onRemove(item.id)}
          >
            {item.label} <span aria-hidden="true">×</span>
          </button>
        ))}
        <input
          type="search"
          aria-label={`Search ${label.toLowerCase()}`}
          role="combobox"
          aria-autocomplete="list"
          aria-expanded={visible}
          aria-controls={visible ? listId : undefined}
          aria-activedescendant={
            visible && options[focusedIndex] ? `${listId}-option-${focusedIndex}` : undefined
          }
          placeholder={placeholder}
          value={query}
          disabled={disabled}
          onFocus={() => onOpenChange(true)}
          onChange={(event) => {
            onQueryChange(event.target.value)
            setActiveIndex(0)
            onOpenChange(true)
          }}
          onKeyDown={(event) => {
            if (event.key === 'Escape' && open) {
              event.preventDefault()
              event.stopPropagation()
              onOpenChange(false)
            }
            if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
              event.preventDefault()
              onOpenChange(true)
              setActiveIndex(
                Math.max(
                  0,
                  Math.min(
                    options.length - 1,
                    visible ? focusedIndex + (event.key === 'ArrowDown' ? 1 : -1) : 0,
                  ),
                ),
              )
            }
            if (event.key === 'Enter' && visible && options[focusedIndex]) {
              event.preventDefault()
              choose(options[focusedIndex].id)
            }
          }}
        />
      </div>
      {visible && (
        <SelectOptions
          id={listId}
          label={label}
          options={options.map((item) => ({
            value: item.id,
            label: item.label,
            description: item.description,
          }))}
          value={selected.map((item) => item.id)}
          activeIndex={focusedIndex}
          onChoose={choose}
          emptyMessage={emptyMessage}
        />
      )}
    </div>
  )
}
