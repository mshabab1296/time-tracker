import type { Notice } from '../../types/notice'

export function Toast({ notice, onDismiss }: { notice: Notice; onDismiss: () => void }) {
  return (
    <div
      className={`toast toast-${notice.type}`}
      role={notice.type === 'error' ? 'alert' : 'status'}
    >
      <span className="toast-icon" aria-hidden="true">
        {notice.type === 'error' ? '!' : '✓'}
      </span>
      <p>{notice.text}</p>
      <button
        type="button"
        className="toast-dismiss"
        aria-label="Dismiss notification"
        onClick={onDismiss}
      >
        ×
      </button>
    </div>
  )
}
