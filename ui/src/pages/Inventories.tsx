import { useState, useEffect } from 'react'
import { useInventories, useOrganizations, useCreateInventory } from '../hooks/useApi'
import { api } from '../services/api'
import { ErrorState } from '../components/ErrorState'
import { PageHeader, EmptyState, LoadingState, Mono } from '../components/Page'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { ListToolbar, ListPagination, useListControls } from '@/hooks/useListControls'
import type { InventoryHost, InventoryGroup } from '../types'

function extractErrorMessage(err: unknown): string {
  if (err && typeof err === 'object' && 'response' in err) {
    const resp = (err as { response?: { data?: { message?: string } } }).response
    if (resp?.data?.message) return resp.data.message
  }
  if (err instanceof Error) return err.message
  return 'An unexpected error occurred'
}

function InventoryDetail({ inventoryId, onClose }: { inventoryId: string; onClose: () => void }) {
  const [tab, setTab] = useState<'hosts' | 'groups'>('hosts')
  const [hosts, setHosts] = useState<InventoryHost[]>([])
  const [groups, setGroups] = useState<InventoryGroup[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [showAddHost, setShowAddHost] = useState(false)
  const [showAddGroup, setShowAddGroup] = useState(false)
  const [hostName, setHostName] = useState('')
  const [hostDesc, setHostDesc] = useState('')
  const [hostVars, setHostVars] = useState('')
  const [groupName, setGroupName] = useState('')
  const [groupDesc, setGroupDesc] = useState('')

  const load = async () => {
    setLoading(true)
    try {
      const [hostsRes, groupsRes] = await Promise.all([
        api.listInventoryHosts(inventoryId),
        api.listInventoryGroups(inventoryId),
      ])
      setHosts(hostsRes.results ?? [])
      setGroups(groupsRes.results ?? [])
      setError(null)
    } catch (err) {
      setError(extractErrorMessage(err))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { load() }, [inventoryId])

  const handleAddHost = async () => {
    try {
      let vars = {}
      if (hostVars.trim()) {
        const parsed: unknown = JSON.parse(hostVars)
        if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
          setError('Variables must be a JSON object')
          return
        }
        vars = parsed as Record<string, unknown>
      }
      await api.createInventoryHost(inventoryId, {
        name: hostName,
        description: hostDesc,
        variables: vars,
      })
      setShowAddHost(false)
      setHostName('')
      setHostDesc('')
      setHostVars('')
      load()
    } catch (err) {
      setError(extractErrorMessage(err))
    }
  }

  const handleAddGroup = async () => {
    try {
      await api.createInventoryGroup(inventoryId, {
        name: groupName,
        description: groupDesc,
      })
      setShowAddGroup(false)
      setGroupName('')
      setGroupDesc('')
      load()
    } catch (err) {
      setError(extractErrorMessage(err))
    }
  }

  return (
    <Card className="mb-6">
      <CardHeader className="flex-row items-center justify-between space-y-0">
        <CardTitle>Inventory Detail</CardTitle>
        <Button variant="secondary" onClick={onClose}>Close</Button>
      </CardHeader>
      <CardContent>
        {error && <ErrorState message={error} />}

        <div className="mb-4 flex gap-2">
          <Button
            variant={tab === 'hosts' ? 'default' : 'secondary'}
            onClick={() => setTab('hosts')}
          >
            Hosts ({hosts.length})
          </Button>
          <Button
            variant={tab === 'groups' ? 'default' : 'secondary'}
            onClick={() => setTab('groups')}
          >
            Groups ({groups.length})
          </Button>
        </div>

        {loading ? (
          <LoadingState />
        ) : tab === 'hosts' ? (
          <div>
            <div className="mb-4">
              <Button onClick={() => setShowAddHost(true)}>+ Add Host</Button>
            </div>

            {showAddHost && (
              <Card className="mb-4 bg-muted/50">
                <CardHeader>
                  <CardTitle className="text-base">Add Host</CardTitle>
                </CardHeader>
                <CardContent className="flex flex-col gap-4">
                  <div className="flex flex-col gap-2">
                    <Label htmlFor="host-name">Name (hostname or IP)</Label>
                    <Input
                      id="host-name"
                      value={hostName}
                      onChange={e => setHostName(e.target.value)}
                      placeholder="web01.example.com"
                    />
                  </div>
                  <div className="flex flex-col gap-2">
                    <Label htmlFor="host-desc">Description</Label>
                    <Input
                      id="host-desc"
                      value={hostDesc}
                      onChange={e => setHostDesc(e.target.value)}
                    />
                  </div>
                  <div className="flex flex-col gap-2">
                    <Label htmlFor="host-vars">Variables (JSON)</Label>
                    <Textarea
                      id="host-vars"
                      className="font-mono text-xs"
                      value={hostVars}
                      onChange={e => setHostVars(e.target.value)}
                      rows={3}
                      placeholder='{"ansible_host": "10.0.0.1"}'
                    />
                  </div>
                  <div className="flex items-center gap-2">
                    <Button onClick={handleAddHost}>Add</Button>
                    <Button variant="secondary" onClick={() => setShowAddHost(false)}>Cancel</Button>
                  </div>
                </CardContent>
              </Card>
            )}

            {hosts.length === 0 ? (
              <EmptyState message="No hosts in this inventory" />
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Name</TableHead>
                    <TableHead>Description</TableHead>
                    <TableHead>Variables</TableHead>
                    <TableHead>Enabled</TableHead>
                    <TableHead>Groups</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {hosts.map(h => (
                    <TableRow key={h.id}>
                      <TableCell><Mono>{h.name}</Mono></TableCell>
                      <TableCell>{h.description || '—'}</TableCell>
                      <TableCell>
                        <Mono>
                          {h.variables && Object.keys(h.variables).length > 0
                            ? Object.keys(h.variables).join(', ')
                            : '—'}
                        </Mono>
                      </TableCell>
                      <TableCell>{h.enabled ? '✅' : '❌'}</TableCell>
                      <TableCell>{h.groups?.length ?? 0}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </div>
        ) : (
          <div>
            <div className="mb-4">
              <Button onClick={() => setShowAddGroup(true)}>+ Add Group</Button>
            </div>

            {showAddGroup && (
              <Card className="mb-4 bg-muted/50">
                <CardHeader>
                  <CardTitle className="text-base">Add Group</CardTitle>
                </CardHeader>
                <CardContent className="flex flex-col gap-4">
                  <div className="flex flex-col gap-2">
                    <Label htmlFor="group-name">Name</Label>
                    <Input
                      id="group-name"
                      value={groupName}
                      onChange={e => setGroupName(e.target.value)}
                      placeholder="web_servers"
                    />
                  </div>
                  <div className="flex flex-col gap-2">
                    <Label htmlFor="group-desc">Description</Label>
                    <Input
                      id="group-desc"
                      value={groupDesc}
                      onChange={e => setGroupDesc(e.target.value)}
                    />
                  </div>
                  <div className="flex items-center gap-2">
                    <Button onClick={handleAddGroup}>Add</Button>
                    <Button variant="secondary" onClick={() => setShowAddGroup(false)}>Cancel</Button>
                  </div>
                </CardContent>
              </Card>
            )}

            {groups.length === 0 ? (
              <EmptyState message="No groups in this inventory" />
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Name</TableHead>
                    <TableHead>Description</TableHead>
                    <TableHead>Hosts</TableHead>
                    <TableHead>Child Groups</TableHead>
                    <TableHead>Variables</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {groups.map(g => (
                    <TableRow key={g.id}>
                      <TableCell><Mono>{g.name}</Mono></TableCell>
                      <TableCell>{g.description || '—'}</TableCell>
                      <TableCell>{g.host_count}</TableCell>
                      <TableCell>{g.child_count}</TableCell>
                      <TableCell>
                        <Mono>
                          {g.variables && Object.keys(g.variables).length > 0
                            ? Object.keys(g.variables).join(', ')
                            : '—'}
                        </Mono>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  )
}

export default function Inventories() {
  const { data: inventories, isLoading, isError, refetch } = useInventories()
  const { data: orgs } = useOrganizations()
  const createInventory = useCreateInventory()
  const [showCreate, setShowCreate] = useState(false)
  const [name, setName] = useState('')
  const [desc, setDesc] = useState('')
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [formError, setFormError] = useState<string | null>(null)

  const list = useListControls(inventories?.results, {
    getText: (inv) =>
      `${inv.name} ${inv.description ?? ''} ${
        orgs?.results?.find((o) => o.id === inv.organization_id)?.name ?? ''
      }`,
    pageSize: 10,
  })

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    setFormError(null)
    try {
      await createInventory.mutateAsync({ name, description: desc })
      setShowCreate(false)
      setName('')
      setDesc('')
    } catch (err) {
      setFormError(extractErrorMessage(err))
    }
  }

  return (
    <div>
      <PageHeader
        title="Inventories"
        actions={<Button onClick={() => setShowCreate(true)}>+ New Inventory</Button>}
      />

      {formError && <ErrorState message={formError} />}

      {showCreate && (
        <Card className="mb-6">
          <CardHeader>
            <CardTitle>Create Inventory</CardTitle>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleCreate} className="flex flex-col gap-4">
              <div className="flex flex-col gap-2">
                <Label htmlFor="inv-name">Name</Label>
                <Input
                  id="inv-name"
                  value={name}
                  onChange={e => setName(e.target.value)}
                  required
                />
              </div>
              <div className="flex flex-col gap-2">
                <Label htmlFor="inv-desc">Description</Label>
                <Input
                  id="inv-desc"
                  value={desc}
                  onChange={e => setDesc(e.target.value)}
                />
              </div>
              <div className="flex items-center gap-2">
                <Button type="submit" disabled={createInventory.isPending}>
                  {createInventory.isPending ? 'Creating...' : 'Create'}
                </Button>
                <Button type="button" variant="secondary" onClick={() => setShowCreate(false)}>
                  Cancel
                </Button>
              </div>
            </form>
          </CardContent>
        </Card>
      )}

      {selectedId && (
        <InventoryDetail inventoryId={selectedId} onClose={() => setSelectedId(null)} />
      )}

      {isLoading ? (
        <LoadingState />
      ) : isError ? (
        <ErrorState message="Failed to load inventories" onRetry={() => refetch()} />
      ) : (
        <>
          <ListToolbar
            query={list.query}
            onQueryChange={list.setQuery}
            placeholder="Search inventories..."
          />
          {!list.pageItems.length ? (
            <Card>
              <CardContent>
                <EmptyState
                  message={list.query ? 'No inventories match your search.' : 'No inventories yet.'}
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
                      <TableHead>Organization</TableHead>
                      <TableHead>Description</TableHead>
                      <TableHead>Hosts</TableHead>
                      <TableHead>Groups</TableHead>
                      <TableHead>Actions</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {list.pageItems.map(inv => (
                      <TableRow key={inv.id}>
                        <TableCell>{inv.name}</TableCell>
                        <TableCell>
                          {orgs?.results?.find(o => o.id === inv.organization_id)?.name ?? inv.organization_id.slice(0, 8)}
                        </TableCell>
                        <TableCell>{inv.description || '—'}</TableCell>
                        <TableCell>{inv.host_count}</TableCell>
                        <TableCell>{inv.group_count}</TableCell>
                        <TableCell>
                          <Button variant="outline" size="sm" onClick={() => setSelectedId(inv.id)}>
                            Manage
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
