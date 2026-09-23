import type { ReactNode } from 'react'

export function ModalFooter({ children }: { children: ReactNode }) {
  return <footer className="modal-footer">{children}</footer>
}
