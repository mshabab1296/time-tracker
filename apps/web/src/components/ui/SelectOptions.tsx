export type SelectOption = { value: string; label: string; description?: string }

export function SelectOptions({
  id,
  label,
  options,
  value,
  activeIndex,
  onChoose,
  emptyMessage,
}: {
  id: string
  label: string
  options: SelectOption[]
  value?: string | string[]
  activeIndex?: number
  onChoose: (value: string) => void
  emptyMessage?: string
}) {
  return (
    <div className="select-options" id={id} role="listbox" aria-label={label}>
      {options.length ? (
        options.map((option, index) => (
          <button
            key={option.value}
            type="button"
            role="option"
            aria-selected={
              Array.isArray(value) ? value.includes(option.value) : value === option.value
            }
            className={activeIndex === index ? 'active' : undefined}
            onMouseDown={(event) => event.preventDefault()}
            onClick={() => onChoose(option.value)}
          >
            <span>{option.label}</span>
            {option.description && <small>{option.description}</small>}
          </button>
        ))
      ) : (
        <span className="muted">{emptyMessage ?? 'No options'}</span>
      )}
    </div>
  )
}
