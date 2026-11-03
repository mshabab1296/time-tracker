// Local-only verification harness: production builds use the root index.html.
import { StrictMode, useState } from 'react'
import { createRoot } from 'react-dom/client'
import { Modal } from '../src/components/ui/Modal'
import { ModalHeader } from '../src/components/ui/ModalHeader'
import { ModalFooter } from '../src/components/ui/ModalFooter'
import { SingleSelect } from '../src/components/ui/SingleSelect'
import { MultiSelect } from '../src/components/ui/MultiSelect'
import '../src/styles/global.css'

const choices = [
  { id: 'one', label: 'Project one' },
  { id: 'two', label: 'Project two' },
  { id: 'three', label: 'Project three' },
]
export function Harness() {
  const [opened, setOpened] = useState(false)
  const [project, setProject] = useState('one')
  const [selected, setSelected] = useState<string[]>([])
  const [query, setQuery] = useState('')
  const [open, setOpen] = useState(false)
  return (
    <main style={{ padding: '24px' }}>
      <h1>Phase 6 component verification</h1>
      <button onClick={() => setOpened(true)}>Open test modal</button>
      <button>Background action</button>
      <output aria-label="Selected project">{project}</output>
      <output aria-label="Selected tags">{selected.join(',')}</output>
      {opened && (
        <Modal titleId="harness-title" onClose={() => setOpened(false)}>
          <ModalHeader
            id="harness-title"
            title="Component verification"
            subtitle="Synthetic data only"
            onClose={() => setOpened(false)}
          />
          <label>
            Project
            <SingleSelect
              label="Project"
              value={project}
              options={choices.map((item) => ({ value: item.id, label: item.label }))}
              onValueChange={setProject}
            />
          </label>
          <label>
            Tags
            <MultiSelect
              label="Tags"
              selected={choices.filter((item) => selected.includes(item.id))}
              options={choices.filter(
                (item) =>
                  !selected.includes(item.id) &&
                  item.label.toLowerCase().includes(query.toLowerCase()),
              )}
              query={query}
              open={open}
              onQueryChange={setQuery}
              onOpenChange={setOpen}
              onSelect={(id) => setSelected((ids) => [...ids, id])}
              onRemove={(id) => setSelected((ids) => ids.filter((value) => value !== id))}
              placeholder="Search tags..."
              emptyMessage="No matching tags"
            />
          </label>
          {Array.from({ length: 25 }, (_, index) => (
            <p key={index}>Scrollable verification row {index + 1}</p>
          ))}
          <ModalFooter>
            <button type="button" className="secondary" onClick={() => setOpened(false)}>
              Close
            </button>
          </ModalFooter>
        </Modal>
      )}
    </main>
  )
}
createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <Harness />
  </StrictMode>,
)
