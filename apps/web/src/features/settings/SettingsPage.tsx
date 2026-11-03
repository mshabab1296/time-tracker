import { useTheme } from '../../app/ThemeContext'
import type { ThemePreference } from '../../app/theme'
import { useSettings } from './useSettings'

export function SettingsPage({ setup = false }: { setup?: boolean }) {
  const { preference, setPreference } = useTheme()
  const {
    saveProfile,
    profileName,
    setProfileName,
    user,
    isAdmin,
    saveOrganization,
    organizationName,
    setOrganizationName,
    organizationTimezone,
    setOrganizationTimezone,
    createOrganization,
    newOrganizationName,
    setNewOrganizationName,
  } = useSettings()

  return (
    <div className="split-layout">
      {!setup && (
        <section className="card">
          <h2>Your profile</h2>
          <form className="stack" onSubmit={(event) => void saveProfile(event)}>
            <label>
              Name
              <input
                value={profileName}
                maxLength={100}
                required
                onChange={(event) => setProfileName(event.target.value)}
              />
            </label>
            <label>
              Timezone
              <input value={user.timezone} disabled />
            </label>
            <button>Save profile</button>
          </form>
        </section>
      )}
      <section className="card">
        <h2>
          {setup
            ? 'Your first organization'
            : isAdmin
              ? 'Organization settings'
              : 'Create an organization'}
        </h2>
        {isAdmin ? (
          <form className="stack" onSubmit={(event) => void saveOrganization(event)}>
            <label>
              Organization name
              <input
                value={organizationName}
                maxLength={100}
                required
                onChange={(event) => setOrganizationName(event.target.value)}
              />
            </label>
            <label>
              Timezone
              <input
                value={organizationTimezone}
                required
                onChange={(event) => setOrganizationTimezone(event.target.value)}
              />
            </label>
            <button>Save organization</button>
          </form>
        ) : null}
        <form
          className="stack create-organization"
          onSubmit={(event) => void createOrganization(event)}
        >
          <h3>
            {setup
              ? 'Choose a name for your workspace'
              : isAdmin
                ? 'Create another organization'
                : 'New organization'}
          </h3>
          <label>
            Organization name
            <input
              value={newOrganizationName}
              maxLength={100}
              required
              placeholder="New organization"
              onChange={(event) => setNewOrganizationName(event.target.value)}
            />
          </label>
          <button className="secondary">Create organization</button>
        </form>
      </section>
      <section className="card">
        <h2>Appearance</h2>
        <p className="muted">System follows this device’s appearance setting.</p>
        <fieldset className="theme-options">
          <legend>Theme</legend>
          {(['system', 'light', 'dark'] as ThemePreference[]).map((option) => (
            <label className="theme-option" key={option}>
              <input
                type="radio"
                name="theme"
                value={option}
                checked={preference === option}
                onChange={() => setPreference(option)}
              />
              <span>{option === 'system' ? 'System' : option === 'light' ? 'Light' : 'Dark'}</span>
            </label>
          ))}
        </fieldset>
      </section>
    </div>
  )
}
