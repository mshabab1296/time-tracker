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
          if (event.key === 'Escape') {
            setOpen(false)
            return
          }
          if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
            event.preventDefault()
            setOpen(true)
            setActiveIndex((index) =>
              Math.max(
                0,
                Math.min(options.length - 1, index + (event.key === 'ArrowDown' ? 1 : -1)),
              ),
            )
          }
          if (event.key === 'Enter' && open && options[activeIndex]) {
            event.preventDefault()
            choose(options[activeIndex].value)
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
          activeIndex={activeIndex}
          onChoose={choose}
        />
      )}
    </div>
  )
}
