import { useEffect, useRef, type ReactNode, type RefObject } from 'react'

export function Modal({
  children,
  titleId,
  onClose,
  className = '',
  dialogRef,
}: {
  children: ReactNode
  titleId: string
  onClose: () => void
  className?: string
  dialogRef?: RefObject<HTMLElement | null>
}) {
  const localRef = useRef<HTMLElement>(null)
  const ref = dialogRef ?? localRef
  const closeRef = useRef(onClose)
  useEffect(() => {
    closeRef.current = onClose
  }, [onClose])
  useEffect(() => {
    const currentDialog = ref.current
    if (!currentDialog) return
    const dialog: HTMLElement = currentDialog
    const previousFocus =
      document.activeElement instanceof HTMLElement ? document.activeElement : null
    const previousOverflow = document.body.style.overflow
    const inertSiblings: { element: HTMLElement; previous: boolean }[] = []
    let branch: HTMLElement | null = dialog.parentElement
    while (branch && branch !== document.body) {
      const parent: HTMLElement | null = branch.parentElement
      if (!parent) break
      for (const sibling of parent.children) {
        if (sibling !== branch && sibling instanceof HTMLElement) {
          inertSiblings.push({ element: sibling, previous: sibling.inert })
          sibling.inert = true
        }
      }
      branch = parent
    }
    document.body.style.overflow = 'hidden'
    const controls = () =>
      [
        ...dialog.querySelectorAll<HTMLElement>(
          'button:not(:disabled), input:not(:disabled), select:not(:disabled), textarea:not(:disabled), a[href], [tabindex]:not([tabindex="-1"])',
        ),
      ].filter(
        (element) =>
          element.tabIndex >= 0 && !element.hidden && element.getClientRects().length > 0,
      )
    if (!dialog.contains(document.activeElement)) controls()[0]?.focus()
    function handleKeyDown(event: KeyboardEvent) {
      if (event.defaultPrevented) return
      if (event.key === 'Escape') {
        event.preventDefault()
        closeRef.current()
        return
      }
      if (event.key !== 'Tab') return
      const items = controls()
      if (!items.length) {
        event.preventDefault()
        dialog.focus()
        return
      }
      const first = items[0],
        last = items[items.length - 1]
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault()
        last.focus()
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault()
        first.focus()
      }
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => {
      document.removeEventListener('keydown', handleKeyDown)
      document.body.style.overflow = previousOverflow
      for (const { element, previous } of inertSiblings) element.inert = previous
      if (previousFocus?.isConnected) previousFocus.focus()
    }
  }, [ref])
  return (
    <div className="modal-backdrop" role="presentation" onMouseDown={onClose}>
      <section
        className={`modal ${className}`.trim()}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        ref={ref}
        tabIndex={-1}
        onMouseDown={(event) => event.stopPropagation()}
      >
        <div className="modal-content">{children}</div>
      </section>
    </div>
  )
}
