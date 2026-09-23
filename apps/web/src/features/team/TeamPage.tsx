import { useTeam } from './useTeam'
import { EmptyState } from '../../components/ui/EmptyState'

export function TeamPage() {
  const {
    isAdmin,
    members,
    removeMember,
    incoming,
    decide,
    invite,
    inviteEmail,
    setInviteEmail,
    sent,
    invitationAction,
  } = useTeam()

  return (
    <div className="split-layout">
      <section className="card">
        <h2>{isAdmin ? 'Members' : 'Your invitations'}</h2>
        {isAdmin ? (
          members.map((member) => (
            <div className="member-row" key={member.id}>
              <span className="avatar small">{member.name.slice(0, 1).toUpperCase()}</span>
              <span>
                <strong>{member.name}</strong>
                <small>
                  {member.email} · {member.role}
                </small>
              </span>
              {member.role === 'MEMBER' && (
                <button className="link" onClick={() => void removeMember(member)}>
                  Remove
                </button>
              )}
            </div>
          ))
        ) : incoming.length ? (
          incoming.map((invitation) => (
            <div className="member-row" key={invitation.id}>
              <span>
                <strong>{invitation.organizationName}</strong>
                <small>Expires {new Date(invitation.expiresAt).toLocaleDateString()}</small>
              </span>
              <span className="row-actions">
                <button onClick={() => void decide(invitation, 'accept')}>Accept</button>
                <button className="secondary" onClick={() => void decide(invitation, 'decline')}>
                  Decline
                </button>
              </span>
            </div>
          ))
        ) : (
          <EmptyState
            title="No invitations"
            text="Organization invitations will appear here."
            compact
          />
        )}
      </section>
      {isAdmin && (
        <section className="card">
          <h2>Invite people</h2>
          <form className="stack" onSubmit={(event) => void invite(event)}>
            <label>
              Email address
              <input
                type="email"
                placeholder="member@example.com"
                required
                value={inviteEmail}
                onChange={(event) => setInviteEmail(event.target.value)}
              />
            </label>
            <button>Send invitation</button>
          </form>
          {sent.length > 0 && (
            <div className="invitation-history">
              <h3>Invitations</h3>
              {sent.map((invitation) => (
                <div className="assignment-row" key={invitation.id}>
                  <span>
                    <strong>{invitation.email}</strong>
                    <small>{invitation.status}</small>
                  </span>
                  {(invitation.status === 'PENDING' || invitation.status === 'EXPIRED') && (
                    <span className="row-actions">
                      <button
                        className="link"
                        onClick={() => void invitationAction(invitation, 'resend')}
                      >
                        Resend
                      </button>
                      {invitation.status === 'PENDING' && (
                        <button
                          className="danger-link"
                          onClick={() => void invitationAction(invitation, 'cancel')}
                        >
                          Cancel
                        </button>
                      )}
                    </span>
                  )}
                </div>
              ))}
            </div>
          )}
        </section>
      )}
    </div>
  )
}
