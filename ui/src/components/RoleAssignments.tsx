import { useState, useEffect } from 'react'
import { api } from '../services/api'
import { ErrorState } from './ErrorState'
import type { Team, User } from '../types'

interface RoleAssignment {
  id: string
  role_name: string
  user_id: string | null
  team_id: string | null
  organization_id: string
  created_at: string
}

function extractErrorMessage(err: unknown): string {
  if (err && typeof err === 'object' && 'response' in err) {
    const resp = (err as { response?: { data?: { message?: string } } }).response
    if (resp?.data?.message) return resp.data.message
  }
  if (err instanceof Error) return err.message
  return 'An unexpected error occurred'
}

export function RoleAssignments({ orgId }: { orgId: string }) {
  const [assignments, setAssignments] = useState<RoleAssignment[]>([])
  const [teams, setTeams] = useState<Team[]>([])
  const [users, setUsers] = useState<User[]>([])
  const [roles, setRoles] = useState<string[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [showAssign, setShowAssign] = useState(false)
  const [selectedRole, setSelectedRole] = useState('')
  const [selectedTeam, setSelectedTeam] = useState('')
  const [selectedUser, setSelectedUser] = useState('')

  const load = async () => {
    setLoading(true)
    try {
      const [assignRes, teamRes, userRes, roleRes] = await Promise.all([
        api.listRoleAssignments(orgId),
        api.listTeams(orgId),
        api.listUsers(),
        api.listRoles(),
      ])
      setAssignments(assignRes.results ?? [])
      setTeams(teamRes.results ?? [])
      setUsers(userRes.results ?? [])
      setRoles((roleRes.results ?? []).map(r => r.name))
      setError(null)
    } catch (err) {
      setError(extractErrorMessage(err))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { load() }, [orgId])

  const handleAssign = async () => {
    try {
      await api.createRoleAssignment(orgId, {
        role_name: selectedRole,
        team_id: selectedTeam || undefined,
        user_id: selectedUser || undefined,
      })
      setShowAssign(false)
      setSelectedRole('')
      setSelectedTeam('')
      setSelectedUser('')
      load()
    } catch (err) {
      setError(extractErrorMessage(err))
    }
  }

  const handleRemove = async (id: string) => {
    try {
      await api.deleteRoleAssignment(orgId, id)
      load()
    } catch (err) {
      setError(extractErrorMessage(err))
    }
  }

  const teamName = (id: string | null) => id ? teams.find(t => t.id === id)?.name ?? id.slice(0, 8) : '—'
  const userName = (id: string | null) => id ? users.find(u => u.id === id)?.email ?? id.slice(0, 8) : '—'

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem' }}>
        <h3 style={{ margin: 0 }}>Role Assignments</h3>
        <button className="btn-primary" onClick={() => setShowAssign(true)}>+ Assign Role</button>
      </div>

      {error && <ErrorState message={error} />}

      {showAssign && (
        <div className="card" style={{ marginBottom: '1rem', background: 'var(--bg-secondary)' }}>
          <h4>Assign Role</h4>
          <div className="form-group">
            <label>Role</label>
            <select value={selectedRole} onChange={e => setSelectedRole(e.target.value)}>
              <option value="">Select role...</option>
              {roles.map(r => (
                <option key={r} value={r}>{r}</option>
              ))}
            </select>
          </div>
          <div className="form-group">
            <label>Assign to Team</label>
            <select value={selectedTeam} onChange={e => { setSelectedTeam(e.target.value); setSelectedUser('') }}>
              <option value="">— Select team —</option>
              {teams.map(t => (
                <option key={t.id} value={t.id}>{t.name}</option>
              ))}
            </select>
          </div>
          <div className="form-group">
            <label>Or assign to User</label>
            <select value={selectedUser} onChange={e => { setSelectedUser(e.target.value); setSelectedTeam('') }}>
              <option value="">— Select user —</option>
              {users.map(u => (
                <option key={u.id} value={u.id}>{u.email}</option>
              ))}
            </select>
          </div>
          <div className="form-actions">
            <button className="btn-primary" onClick={handleAssign} disabled={!selectedRole || (!selectedTeam && !selectedUser)}>
              Assign
            </button>
            <button className="btn-secondary" onClick={() => setShowAssign(false)}>Cancel</button>
          </div>
        </div>
      )}

      {loading ? (
        <div className="loading">Loading...</div>
      ) : assignments.length === 0 ? (
        <div className="empty-state">No role assignments yet</div>
      ) : (
        <table className="table">
          <thead>
            <tr>
              <th>Role</th>
              <th>Assigned To</th>
              <th>Type</th>
              <th>Created</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {assignments.map(a => (
              <tr key={a.id}>
                <td><span className="status-badge status-active">{a.role_name}</span></td>
                <td>{a.team_id ? teamName(a.team_id) : userName(a.user_id)}</td>
                <td>{a.team_id ? 'Team' : 'User'}</td>
                <td>{new Date(a.created_at).toLocaleDateString()}</td>
                <td>
                  <button className="btn-sm btn-danger" onClick={() => handleRemove(a.id)}>
                    Remove
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}
