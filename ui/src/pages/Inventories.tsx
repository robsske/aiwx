import { useState } from 'react'
import { useInventories, useCreateInventory } from '../hooks/useApi'

export default function Inventories() {
  const { data: inventories, isLoading } = useInventories()
  const createInventory = useCreateInventory()
  const [showCreate, setShowCreate] = useState(false)
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    await createInventory.mutateAsync({ name, description })
    setShowCreate(false)
    setName('')
    setDescription('')
  }

  return (
    <div className="page">
      <div className="page-header">
        <h1 className="page-title">Inventories</h1>
        <button className="btn-primary" onClick={() => setShowCreate(true)}>
          + New Inventory
        </button>
      </div>

      {showCreate && (
        <div className="card">
          <h2 className="card-title">Create Inventory</h2>
          <form onSubmit={handleCreate} className="form">
            <div className="form-group">
              <label>Name</label>
              <input type="text" value={name} onChange={(e) => setName(e.target.value)} required />
            </div>
            <div className="form-group">
              <label>Description</label>
              <input
                type="text"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
              />
            </div>
            <div className="form-actions">
              <button type="submit" className="btn-primary" disabled={createInventory.isPending}>
                Create
              </button>
              <button type="button" className="btn-secondary" onClick={() => setShowCreate(false)}>
                Cancel
              </button>
            </div>
          </form>
        </div>
      )}

      {isLoading ? (
        <div className="loading">Loading...</div>
      ) : (
        <div className="card">
          <table className="table">
            <thead>
              <tr>
                <th>Name</th>
                <th>Description</th>
                <th>Hosts</th>
                <th>Groups</th>
                <th>Created</th>
              </tr>
            </thead>
            <tbody>
              {inventories?.results?.map((inv) => (
                <tr key={inv.id}>
                  <td>{inv.name}</td>
                  <td>{inv.description || '—'}</td>
                  <td>{inv.host_count}</td>
                  <td>{inv.group_count}</td>
                  <td>{new Date(inv.created_at).toLocaleDateString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
