import { useState } from 'react'
import { useInventories, useOrganizations, useCreateInventory } from '../hooks/useApi'
import { api } from '../services/api'
import { ErrorState } from '../components/ErrorState'
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

  useState(() => { load() })

  const handleAddHost = async () => {
    try {
      let vars = {}
      if (hostVars.trim()) vars = JSON.parse(hostVars)
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
    <div className="card">
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem' }}>
        <h2 className="card-title" style={{ margin: 0 }}>Inventory Detail</h2>
        <button className="btn-secondary" onClick={onClose}>Close</button>
      </div>

      {error && <ErrorState message={error} />}

      <div style={{ display: 'flex', gap: '0.5rem', marginBottom: '1rem' }}>
        <button
          className={tab === 'hosts' ? 'btn-primary' : 'btn-secondary'}
          onClick={() => setTab('hosts')}
        >
          Hosts ({hosts.length})
        </button>
        <button
          className={tab === 'groups' ? 'btn-primary' : 'btn-secondary'}
          onClick={() => setTab('groups')}
        >
          Groups ({groups.length})
        </button>
      </div>

      {loading ? (
        <div className="loading">Loading...</div>
      ) : tab === 'hosts' ? (
        <div>
          <div style={{ marginBottom: '1rem' }}>
            <button className="btn-primary" onClick={() => setShowAddHost(true)}>+ Add Host</button>
          </div>

          {showAddHost && (
            <div className="card" style={{ marginBottom: '1rem', background: 'var(--bg-secondary)' }}>
              <h3>Add Host</h3>
              <div className="form-group">
                <label>Name (hostname or IP)</label>
                <input value={hostName} onChange={e => setHostName(e.target.value)} placeholder="web01.example.com" />
              </div>
              <div className="form-group">
                <label>Description</label>
                <input value={hostDesc} onChange={e => setHostDesc(e.target.value)} />
              </div>
              <div className="form-group">
                <label>Variables (JSON)</label>
                <textarea value={hostVars} onChange={e => setHostVars(e.target.value)} rows={3} placeholder='{"ansible_host": "10.0.0.1"}' />
              </div>
              <div className="form-actions">
                <button className="btn-primary" onClick={handleAddHost}>Add</button>
                <button className="btn-secondary" onClick={() => setShowAddHost(false)}>Cancel</button>
              </div>
            </div>
          )}

          {hosts.length === 0 ? (
            <div className="empty-state">No hosts in this inventory</div>
          ) : (
            <table className="table">
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Description</th>
                  <th>Variables</th>
                  <th>Enabled</th>
                  <th>Groups</th>
                </tr>
              </thead>
              <tbody>
                {hosts.map(h => (
                  <tr key={h.id}>
                    <td className="mono">{h.name}</td>
                    <td>{h.description || '—'}</td>
                    <td className="mono" style={{ fontSize: '0.75rem' }}>
                      {h.variables && Object.keys(h.variables).length > 0
                        ? Object.keys(h.variables).join(', ')
                        : '—'}
                    </td>
                    <td>{h.enabled ? '✅' : '❌'}</td>
                    <td>{h.groups?.length ?? 0}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      ) : (
        <div>
          <div style={{ marginBottom: '1rem' }}>
            <button className="btn-primary" onClick={() => setShowAddGroup(true)}>+ Add Group</button>
          </div>

          {showAddGroup && (
            <div className="card" style={{ marginBottom: '1rem', background: 'var(--bg-secondary)' }}>
              <h3>Add Group</h3>
              <div className="form-group">
                <label>Name</label>
                <input value={groupName} onChange={e => setGroupName(e.target.value)} placeholder="web_servers" />
              </div>
              <div className="form-group">
                <label>Description</label>
                <input value={groupDesc} onChange={e => setGroupDesc(e.target.value)} />
              </div>
              <div className="form-actions">
                <button className="btn-primary" onClick={handleAddGroup}>Add</button>
                <button className="btn-secondary" onClick={() => setShowAddGroup(false)}>Cancel</button>
              </div>
            </div>
          )}

          {groups.length === 0 ? (
            <div className="empty-state">No groups in this inventory</div>
          ) : (
            <table className="table">
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Description</th>
                  <th>Hosts</th>
                  <th>Child Groups</th>
                  <th>Variables</th>
                </tr>
              </thead>
              <tbody>
                {groups.map(g => (
                  <tr key={g.id}>
                    <td className="mono">{g.name}</td>
                    <td>{g.description || '—'}</td>
                    <td>{g.host_count}</td>
                    <td>{g.child_count}</td>
                    <td className="mono" style={{ fontSize: '0.75rem' }}>
                      {g.variables && Object.keys(g.variables).length > 0
                        ? Object.keys(g.variables).join(', ')
                        : '—'}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}
    </div>
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
    <div className="page">
      <div className="page-header">
        <h1 className="page-title">Inventories</h1>
        <button className="btn-primary" onClick={() => setShowCreate(true)}>
          + New Inventory
        </button>
      </div>

      {formError && <ErrorState message={formError} />}

      {showCreate && (
        <div className="card">
          <h2 className="card-title">Create Inventory</h2>
          <form onSubmit={handleCreate} className="form">
            <div className="form-group">
              <label>Name</label>
              <input value={name} onChange={e => setName(e.target.value)} required />
            </div>
            <div className="form-group">
              <label>Description</label>
              <input value={desc} onChange={e => setDesc(e.target.value)} />
            </div>
            <div className="form-actions">
              <button type="submit" className="btn-primary" disabled={createInventory.isPending}>
                {createInventory.isPending ? 'Creating...' : 'Create'}
              </button>
              <button type="button" className="btn-secondary" onClick={() => setShowCreate(false)}>Cancel</button>
            </div>
          </form>
        </div>
      )}

      {selectedId && (
        <InventoryDetail inventoryId={selectedId} onClose={() => setSelectedId(null)} />
      )}

      {isLoading ? (
        <div className="loading">Loading...</div>
      ) : isError ? (
        <ErrorState message="Failed to load inventories" onRetry={() => refetch()} />
      ) : (
        <div className="card">
          <table className="table">
            <thead>
              <tr>
                <th>Name</th>
                <th>Organization</th>
                <th>Description</th>
                <th>Hosts</th>
                <th>Groups</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {inventories?.results?.map(inv => (
                <tr key={inv.id}>
                  <td>{inv.name}</td>
                  <td>{orgs?.results?.find(o => o.id === inv.organization_id)?.name ?? inv.organization_id.slice(0, 8)}</td>
                  <td>{inv.description || '—'}</td>
                  <td>{inv.host_count}</td>
                  <td>{inv.group_count}</td>
                  <td>
                    <button className="btn-sm" onClick={() => setSelectedId(inv.id)}>
                      Manage
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
