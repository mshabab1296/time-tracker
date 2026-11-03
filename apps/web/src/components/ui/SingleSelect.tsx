import { useId, useState } from 'react'
import { SelectOptions, type SelectOption } from './SelectOptions'

type Props = {
  value: string
  options: SelectOption[]
  onValueChange: (value: string) => void
  label: string
  disabled?: boolean
  placeholder?: string
  required?: boolean
}

export function SingleSelect({
  value,
  options,
  onValueChange,
  label,
  disabled = false,
  placeholder = 'Select…',
  required = false,
}: Props) {
  const [open, setOpen] = useState(false)
  const [activeIndex, setActiveIndex] = useState(0)
  const listId = useId()
  const selected = options.find((option) => option.value === value)
  const focusedIndex = Math.max(0, Math.min(activeIndex, options.length - 1))

  function choose(nextValue: string) {
    onValueChange(nextValue)
    setOpen(false)
  }

  return (
    <div
      className="single-select"
      onBlur={(event) => {
        if (!event.currentTarget.contains(event.relatedTarget)) setOpen(false)
      }}
    >
      <button
        type="button"
        className="single-select-field"
        role="combobox"
        aria-autocomplete="none"
        aria-activedescendant={
          open && options[focusedIndex] ? `${listId}-option-${focusedIndex}` : undefined
        }
        aria-label={label}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-controls={open ? listId : undefined}
        aria-required={required}
        disabled={disabled}
        onClick={() => {
          setActiveIndex(
            Math.max(
              0,
              options.findIndex((option) => option.value === value),
            ),
          )
          setOpen((current) => !current)
        }}
        onKeyDown={(event) => {
          if (event.key === 'Escape' && open) {
            event.preventDefault()
            event.stopPropagation()
            setOpen(false)
            return
          }
          if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
            event.preventDefault()
            setOpen(true)
            setActiveIndex(
              Math.max(
                0,
                Math.min(
                  options.length - 1,
                  open
                    ? focusedIndex + (event.key === 'ArrowDown' ? 1 : -1)
                    : Math.max(
                        0,
                        options.findIndex((option) => option.value === value),
                      ),
                ),
              ),
            )
          }
          if (open && (event.key === 'Home' || event.key === 'End')) {
            event.preventDefault()
            setActiveIndex(event.key === 'Home' ? 0 : options.length - 1)
          }
          if ((event.key === 'Enter' || event.key === ' ') && open && options[focusedIndex]) {
            event.preventDefault()
            choose(options[focusedIndex].value)
          }
        }}
      >
        <span className={selected ? '' : 'placeholder'}>{selected?.label ?? placeholder}</span>
        <svg className="select-chevron" aria-hidden="true" viewBox="0 0 20 20" fill="none">
          <path
            d="m5 7.5 5 5 5-5"
            stroke="currentColor"
            strokeWidth="1.8"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </svg>
      </button>
      {open && (
        <SelectOptions
          id={listId}
          label={label}
          options={options}
          value={value}
          activeIndex={focusedIndex}
          onChoose={choose}
        />
      )}
    </div>
  )
}
