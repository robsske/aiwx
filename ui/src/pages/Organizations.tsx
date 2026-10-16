import { useState } from 'react'
import { api } from '../services/api'
import { ErrorState } from '../components/ErrorState'
import { RoleAssignments } from '../components/RoleAssignments'
import type { Organization, Team } from '../types'

function extractErrorMessage(err: unknown): string {
  if (err && typeof err === 'object' && 'response' in err) {
    const resp = (err as { response?: { data?: { message?: string } } }).response
    if (resp?.data?.message) return resp.data.message
  }
  if (err instanceof Error) return err.message
  return 'An unexpected error occurred'
}

function TeamsPanel({ orgId }: { orgId: string }) {
  const [teams, setTeams] = useState<Team[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [showCreate, setShowCreate] = useState(false)
  const [teamName, setTeamName] = useState('')
  const [teamDesc, setTeamDesc] = useState('')

  const load = async () => {
    setLoading(true)
    try {
      const res = await api.listTeams(orgId)
      setTeams(res.results ?? [])
      setError(null)
    } catch (err) {
      setError(extractErrorMessage(err))
    } finally {
      setLoading(false)
    }
  }

  useState(() => { load() })

  const handleCreate = async () => {
    try {
      await api.createTeam(orgId, { name: teamName, display_name: teamDesc })
      setShowCreate(false)
      setTeamName('')
      setTeamDesc('')
      load()
    } catch (err) {
      setError(extractErrorMessage(err))
    }
  }

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem' }}>
        <h3 style={{ margin: 0 }}>Teams</h3>
        <button className="btn-primary" onClick={() => setShowCreate(true)}>+ Add Team</button>
      </div>

      {error && <ErrorState message={error} />}

      {showCreate && (
        <div className="card" style={{ marginBottom: '1rem', background: 'var(--bg-secondary)' }}>
          <h4>Create Team</h4>
          <div className="form-group">
            <label>Name</label>
            <input value={teamName} onChange={e => setTeamName(e.target.value)} placeholder="ops-team" />
          </div>
          <div className="form-group">
            <label>Display Name</label>
            <input value={teamDesc} onChange={e => setTeamDesc(e.target.value)} placeholder="Operations Team" />
          </div>
          <div className="form-actions">
            <button className="btn-primary" onClick={handleCreate}>Create</button>
            <button className="btn-secondary" onClick={() => setShowCreate(false)}>Cancel</button>
          </div>
        </div>
      )}

      {loading ? (
        <div className="loading">Loading...</div>
      ) : teams.length === 0 ? (
        <div className="empty-state">No teams in this organization</div>
      ) : (
        <table className="table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Display Name</th>
              <th>Members</th>
            </tr>
          </thead>
          <tbody>
            {teams.map(t => (
              <tr key={t.id}>
                <td className="mono">{t.name}</td>
                <td>{t.display_name || '—'}</td>
                <td>{t.member_count}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}

export default function Organizations() {
  const [orgs, setOrgs] = useState<Organization[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [showCreate, setShowCreate] = useState(false)
  const [name, setName] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [selectedOrg, setSelectedOrg] = useState<string | null>(null)

  const load = async () => {
    setLoading(true)
    try {
      const res = await api.listOrganizations()
      setOrgs(res.results ?? [])
      setError(null)
    } catch (err) {
      setError(extractErrorMessage(err))
    } finally {
      setLoading(false)
    }
  }

  useState(() => { load() })

  const handleCreate = async () => {
    try {
      await api.createOrganization({ name, display_name: displayName })
      setShowCreate(false)
      setName('')
      setDisplayName('')
      load()
    } catch (err) {
      setError(extractErrorMessage(err))
    }
  }

  return (
    <div className="page">
      <div className="page-header">
        <h1 className="page-title">Organizations</h1>
        <button className="btn-primary" onClick={() => setShowCreate(true)}>
          + New Organization
        </button>
      </div>

      {error && <ErrorState message={error} />}

      {showCreate && (
        <div className="card">
          <h2 className="card-title">Create Organization</h2>
          <div className="form">
            <div className="form-group">
              <label>Name (slug)</label>
              <input value={name} onChange={e => setName(e.target.value)} placeholder="acme-corp" />
            </div>
            <div className="form-group">
              <label>Display Name</label>
              <input value={displayName} onChange={e => setDisplayName(e.target.value)} placeholder="ACME Corporation" />
            </div>
            <div className="form-actions">
              <button className="btn-primary" onClick={handleCreate}>Create</button>
              <button className="btn-secondary" onClick={() => setShowCreate(false)}>Cancel</button>
            </div>
          </div>
        </div>
      )}

      {selectedOrg && (
        <div className="card">
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem' }}>
            <h2 className="card-title" style={{ margin: 0 }}>Organization Detail</h2>
            <button className="btn-secondary" onClick={() => setSelectedOrg(null)}>Close</button>
          </div>
          <TeamsPanel orgId={selectedOrg} />
          <hr style={{ margin: '1.5rem 0', border: 'none', borderTop: '1px solid var(--border)' }} />
          <RoleAssignments orgId={selectedOrg} />
        </div>
      )}

      {loading ? (
        <div className="loading">Loading...</div>
      ) : (
        <div className="card">
          <table className="table">
            <thead>
              <tr>
                <th>Name</th>
                <th>Display Name</th>
                <th>Status</th>
                <th>Created</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {orgs.map(org => (
                <tr key={org.id}>
                  <td className="mono">{org.name}</td>
                  <td>{org.display_name || '—'}</td>
                  <td>
                    <span className={`status-badge status-${org.status}`}>{org.status}</span>
                  </td>
                  <td>{new Date(org.created_at).toLocaleDateString()}</td>
                  <td>
                    <button className="btn-sm" onClick={() => setSelectedOrg(org.id)}>
                      Manage Teams
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
