import { useState } from 'react'
import { api } from '../services/api'
import { ErrorState } from '../components/ErrorState'
import { RoleAssignments } from '../components/RoleAssignments'
import { PageHeader, StatusBadge, EmptyState, LoadingState, Mono } from '../components/Page'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Separator } from '@/components/ui/separator'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { ListToolbar, ListPagination, useListControls } from '@/hooks/useListControls'
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
      <div className="mb-4 flex items-center justify-between">
        <h3 className="text-lg font-semibold">Teams</h3>
        <Button onClick={() => setShowCreate(true)}>+ Add Team</Button>
      </div>

      {error && <ErrorState message={error} />}

      {showCreate && (
        <Card className="mb-4">
          <CardHeader>
            <CardTitle className="text-base">Create Team</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="team-name">Name</Label>
              <Input
                id="team-name"
                value={teamName}
                onChange={e => setTeamName(e.target.value)}
                placeholder="ops-team"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="team-display-name">Display Name</Label>
              <Input
                id="team-display-name"
                value={teamDesc}
                onChange={e => setTeamDesc(e.target.value)}
                placeholder="Operations Team"
              />
            </div>
            <div className="flex gap-2">
              <Button onClick={handleCreate}>Create</Button>
              <Button variant="outline" onClick={() => setShowCreate(false)}>Cancel</Button>
            </div>
          </CardContent>
        </Card>
      )}

      {loading ? (
        <LoadingState />
      ) : teams.length === 0 ? (
        <EmptyState message="No teams in this organization" />
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Display Name</TableHead>
              <TableHead>Members</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {teams.map(t => (
              <TableRow key={t.id}>
                <TableCell><Mono>{t.name}</Mono></TableCell>
                <TableCell>{t.display_name || '—'}</TableCell>
                <TableCell>{t.member_count}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
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

  const list = useListControls(orgs, {
    getText: (o) => `${o.name} ${o.display_name ?? ''} ${o.status}`,
    pageSize: 10,
  })

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
    <div>
      <PageHeader
        title="Organizations"
        actions={<Button onClick={() => setShowCreate(true)}>+ New Organization</Button>}
      />

      {error && <ErrorState message={error} />}

      {showCreate && (
        <Card className="mb-6">
          <CardHeader>
            <CardTitle>Create Organization</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="org-name">Name (slug)</Label>
              <Input
                id="org-name"
                value={name}
                onChange={e => setName(e.target.value)}
                placeholder="acme-corp"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="org-display-name">Display Name</Label>
              <Input
                id="org-display-name"
                value={displayName}
                onChange={e => setDisplayName(e.target.value)}
                placeholder="ACME Corporation"
              />
            </div>
            <div className="flex gap-2">
              <Button onClick={handleCreate}>Create</Button>
              <Button variant="outline" onClick={() => setShowCreate(false)}>Cancel</Button>
            </div>
          </CardContent>
        </Card>
      )}

      {selectedOrg && (
        <Card className="mb-6">
          <CardHeader>
            <div className="flex items-center justify-between">
              <CardTitle>Organization Detail</CardTitle>
              <Button variant="outline" onClick={() => setSelectedOrg(null)}>Close</Button>
            </div>
          </CardHeader>
          <CardContent className="space-y-6">
            <TeamsPanel orgId={selectedOrg} />
            <Separator />
            <RoleAssignments orgId={selectedOrg} />
          </CardContent>
        </Card>
      )}

      {loading ? (
        <LoadingState />
      ) : (
        <>
          <ListToolbar
            query={list.query}
            onQueryChange={list.setQuery}
            placeholder="Search organizations..."
          />
          {!list.pageItems.length ? (
            <Card>
              <CardContent>
                <EmptyState
                  message={list.query ? 'No organizations match your search.' : 'No organizations yet.'}
                />
              </CardContent>
            </Card>
          ) : (
            <Card>
              <CardContent className="p-0">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Name</TableHead>
                      <TableHead>Display Name</TableHead>
                      <TableHead>Status</TableHead>
                      <TableHead>Created</TableHead>
                      <TableHead>Actions</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {list.pageItems.map(org => (
                      <TableRow key={org.id}>
                        <TableCell><Mono>{org.name}</Mono></TableCell>
                        <TableCell>{org.display_name || '—'}</TableCell>
                        <TableCell>
                          <StatusBadge status={org.status} />
                        </TableCell>
                        <TableCell>{new Date(org.created_at).toLocaleDateString()}</TableCell>
                        <TableCell>
                          <Button size="sm" variant="outline" onClick={() => setSelectedOrg(org.id)}>
                            Manage Teams
                          </Button>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>
          )}
          <ListPagination
            page={list.page}
            totalPages={list.totalPages}
            pageSize={list.pageSize}
            total={list.total}
            rangeLabel={list.rangeLabel}
            onPageChange={list.setPage}
            onPageSizeChange={list.setPageSize}
          />
        </>
      )}
    </div>
  )
}
