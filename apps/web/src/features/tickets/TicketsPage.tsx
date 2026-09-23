import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { api } from '../../api/client'
import { IconButton } from '../../components/ui/IconButton'
import type { Ticket, TicketPage } from '../../types/domain'
import { useWorkspace } from '../dashboard/WorkspaceContext'

export function TicketsPage() {
  const { organizationID, user, isAdmin, setMessage } = useWorkspace()
  const [page, setPage] = useState<TicketPage | null>(null)
  const [query, setQuery] = useState('')
  const [offset, setOffset] = useState(0)
  const [reference, setReference] = useState('')
  const [title, setTitle] = useState('')
  const [editing, setEditing] = useState<Ticket | null>(null)
  const [busy, setBusy] = useState(false)

  const load = useCallback(async () => {
    if (!organizationID) return
    const parameters = new URLSearchParams({ query, limit: '25', offset: String(offset) })
    const result = await api<TicketPage>(`/organizations/${organizationID}/tickets?${parameters}`)
    if (result.error) setMessage(result.error.message, 'error')
    else setPage(result.data)
  }, [organizationID, query, offset, setMessage])

  useEffect(() => {
    void load()
  }, [load])

  async function save(event: FormEvent) {
    event.preventDefault()
    if (!organizationID) return
    setBusy(true)
    const path = `/organizations/${organizationID}/tickets${editing ? `/${editing.id}` : ''}`
    const result = await api<Ticket>(path, {
      method: editing ? 'PATCH' : 'POST',
      body: JSON.stringify({ reference: reference.trim(), title: title.trim() }),
    })
    setBusy(false)
    if (result.error) {
      setMessage(result.error.message, 'error')
      return
    }
    setEditing(null)
    setReference('')
    setTitle('')
    setMessage(editing ? 'Ticket updated.' : 'Ticket added.')
    void load()
  }

  async function remove(ticket: Ticket) {
    if (!organizationID || !window.confirm(`Delete ${ticket.reference}?`)) return
    const result = await api<{ deleted: boolean }>(
      `/organizations/${organizationID}/tickets/${ticket.id}`,
      { method: 'DELETE' },
    )
    if (result.error) setMessage(result.error.message, 'error')
    else {
      setMessage('Ticket deleted.')
      void load()
    }
  }

  return (
    <div className="card tickets-page">
      <div className="section-heading">
        <div>
          <h2>Tickets</h2>
          <p>
            Ticket references are shared within this organization and can be used with any project.
          </p>
        </div>
        <span className="count-chip">{page?.total ?? 0}</span>
      </div>
      <form className="ticket-form" onSubmit={(event) => void save(event)}>
        <label>
          Reference
          <input
            value={reference}
            onChange={(event) => setReference(event.target.value)}
            maxLength={100}
            placeholder="PROJ-123"
            required
          />
        </label>
        <label>
          Title
          <input
            value={title}
            onChange={(event) => setTitle(event.target.value)}
            maxLength={200}
            placeholder="Investigate login issue"
            required
          />
        </label>
        <button disabled={busy || !reference.trim() || !title.trim()}>
          {editing ? 'Save' : 'Add'}
        </button>
        {editing && (
          <button
            type="button"
            className="secondary"
            onClick={() => {
              setEditing(null)
              setReference('')
              setTitle('')
            }}
          >
            Cancel
          </button>
        )}
      </form>
      <label className="ticket-search">
        Search tickets
        <input
          value={query}
          onChange={(event) => {
            setQuery(event.target.value)
            setOffset(0)
          }}
          placeholder="Reference or title"
        />
      </label>
      <div className="entry-list">
        {page?.items.map((ticket) => (
          <div className="entry-row" key={ticket.id}>
            <div>
              <strong>{ticket.reference}</strong>
              <small>{ticket.title}</small>
            </div>
            {(isAdmin || ticket.createdByUserId === user.id) && (
              <span className="row-actions">
                <IconButton
                  label={`Edit ${ticket.reference}`}
                  icon="✎"
                  onClick={() => {
                    setEditing(ticket)
                    setReference(ticket.reference)
                    setTitle(ticket.title)
                  }}
                />
                <IconButton
                  label={`Delete ${ticket.reference}`}
                  icon="⌫"
                  onClick={() => void remove(ticket)}
                />
              </span>
            )}
          </div>
        ))}
      </div>
      {page?.items.length === 0 && <p className="muted">No tickets found.</p>}
      {page && page.total > page.limit && (
        <div className="pagination">
          <button
            className="secondary"
            disabled={offset === 0}
            onClick={() => setOffset(Math.max(0, offset - 25))}
          >
            Previous
          </button>
          <span>
            {offset + 1}–{Math.min(offset + 25, page.total)} of {page.total}
          </span>
          <button
            className="secondary"
            disabled={offset + 25 >= page.total}
            onClick={() => setOffset(offset + 25)}
          >
            Next
          </button>
        </div>
      )}
    </div>
  )
}
