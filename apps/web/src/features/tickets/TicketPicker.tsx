import { useEffect, useState } from 'react'
import { api } from '../../api/client'
import type { Ticket, TicketPage } from '../../types/domain'
import { MultiSelect } from '../../components/ui/MultiSelect'

type Props = {
  organizationID: string
  value: string[]
  onChange: (ids: string[]) => void
  disabled?: boolean
}

export function TicketPicker({ organizationID, value, onChange, disabled }: Props) {
  const [selected, setSelected] = useState<Record<string, Ticket>>({})
  const [options, setOptions] = useState<Ticket[]>([])
  const [query, setQuery] = useState('')
  const [open, setOpen] = useState(false)

  useEffect(() => {
    if (!value.length || !organizationID) return
    let cancelled = false
    for (const id of value) {
      void api<Ticket>(`/organizations/${organizationID}/tickets/${id}`).then((result) => {
        if (!cancelled && !result.error)
          setSelected((current) => ({ ...current, [id]: result.data }))
      })
    }
    return () => {
      cancelled = true
    }
  }, [organizationID, value])

  useEffect(() => {
    if (!open || !organizationID) return
    let cancelled = false
    const timeout = window.setTimeout(
      () => {
        const parameters = new URLSearchParams({ limit: query ? '20' : '5', query })
        void api<TicketPage>(`/organizations/${organizationID}/tickets?${parameters}`).then(
          (result) => {
            if (!cancelled && !result.error) setOptions(result.data.items)
          },
        )
      },
      query ? 250 : 0,
    )
    return () => {
      cancelled = true
      window.clearTimeout(timeout)
    }
  }, [organizationID, query, open])

  return (
    <MultiSelect
      label="Tickets"
      selected={value.map((id) => ({ id, label: selected[id]?.reference ?? 'Selected ticket' }))}
      options={options
        .filter((ticket) => !value.includes(ticket.id))
        .map((ticket) => ({ id: ticket.id, label: ticket.reference, description: ticket.title }))}
      query={query}
      open={open}
      onQueryChange={setQuery}
      onOpenChange={setOpen}
      onSelect={(id) => {
        onChange([...value, id])
        const ticket = options.find((item) => item.id === id)
        if (ticket) setSelected((current) => ({ ...current, [id]: ticket }))
      }}
      onRemove={(id) => onChange(value.filter((item) => item !== id))}
      placeholder={value.length ? 'Add ticket…' : 'Search tickets…'}
      emptyMessage="No matching tickets"
      disabled={disabled}
      closeOnSelect
      showEmptyOnOpen
    />
  )
}
