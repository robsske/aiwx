import { useState, useEffect } from 'react'
import { api } from '../services/api'
import { ErrorState } from './ErrorState'
import { EmptyState, LoadingState } from './Page'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
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

const NONE = '__none__'

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
      <div className="mb-4 flex items-center justify-between">
        <h3 className="text-lg font-semibold">Role Assignments</h3>
        <Button onClick={() => setShowAssign(true)}>+ Assign Role</Button>
      </div>

      {error && <ErrorState message={error} />}

      {showAssign && (
        <Card className="mb-4">
          <CardHeader>
            <CardTitle className="text-base">Assign Role</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label>Role</Label>
              <Select
                value={selectedRole || NONE}
                onValueChange={(value: string) => setSelectedRole(value === NONE ? '' : value)}
              >
                <SelectTrigger>
                  <SelectValue placeholder="Select role..." />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={NONE}>Select role...</SelectItem>
                  {roles.map(r => (
                    <SelectItem key={r} value={r}>{r}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label>Assign to Team</Label>
              <Select
                value={selectedTeam || NONE}
                onValueChange={(value: string) => {
                  setSelectedTeam(value === NONE ? '' : value)
                  setSelectedUser('')
                }}
              >
                <SelectTrigger>
                  <SelectValue placeholder="— Select team —" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={NONE}>— Select team —</SelectItem>
                  {teams.map(t => (
                    <SelectItem key={t.id} value={t.id}>{t.name}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label>Or assign to User</Label>
              <Select
                value={selectedUser || NONE}
                onValueChange={(value: string) => {
                  setSelectedUser(value === NONE ? '' : value)
                  setSelectedTeam('')
                }}
              >
                <SelectTrigger>
                  <SelectValue placeholder="— Select user —" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={NONE}>— Select user —</SelectItem>
                  {users.map(u => (
                    <SelectItem key={u.id} value={u.id}>{u.email}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="flex gap-2">
              <Button
                onClick={handleAssign}
                disabled={!selectedRole || (!selectedTeam && !selectedUser)}
              >
                Assign
              </Button>
              <Button variant="outline" onClick={() => setShowAssign(false)}>Cancel</Button>
            </div>
          </CardContent>
        </Card>
      )}

      {loading ? (
        <LoadingState />
      ) : assignments.length === 0 ? (
        <EmptyState message="No role assignments yet" />
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Role</TableHead>
              <TableHead>Assigned To</TableHead>
              <TableHead>Type</TableHead>
              <TableHead>Created</TableHead>
              <TableHead>Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {assignments.map(a => (
              <TableRow key={a.id}>
                <TableCell><Badge variant="secondary">{a.role_name}</Badge></TableCell>
                <TableCell>
                  {a.team_id ? teamName(a.team_id) : userName(a.user_id)}
                </TableCell>
                <TableCell>{a.team_id ? 'Team' : 'User'}</TableCell>
                <TableCell>{new Date(a.created_at).toLocaleDateString()}</TableCell>
                <TableCell>
                  <Button size="sm" variant="destructive" onClick={() => handleRemove(a.id)}>
                    Remove
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </div>
  )
}
