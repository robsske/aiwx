import { useState } from 'react'
import { useCredentials, useCredentialTypes, useCreateCredential, useDeleteCredential } from '../hooks/useApi'

export default function Credentials() {
  const { data: credentials, isLoading } = useCredentials()
  const { data: credentialTypes } = useCredentialTypes()
  const createCredential = useCreateCredential()
  const deleteCredential = useDeleteCredential()
  const [showCreate, setShowCreate] = useState(false)
  const [name, setName] = useState('')
  const [typeId, setTypeId] = useState('')
  const [inputs, setInputs] = useState('')

  const selectedType = credentialTypes?.find((t) => t.id === typeId)

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    let parsedInputs: Record<string, unknown>
    try {
      parsedInputs = inputs.trim() ? JSON.parse(inputs) : {}
    } catch {
      alert('Invalid JSON for inputs')
      return
    }
    try {
      await createCredential.mutateAsync({
        name,
        credential_type_id: typeId,
        inputs: parsedInputs,
      })
      setShowCreate(false)
      setName('')
      setInputs('')
    } catch {
      alert('Failed to create credential')
    }
  }

  return (
    <div className="page">
      <div className="page-header">
        <h1 className="page-title">Credentials</h1>
        <button className="btn-primary" onClick={() => setShowCreate(true)}>
          + New Credential
        </button>
      </div>

      {showCreate && (
        <div className="card">
          <h2 className="card-title">Create Credential</h2>
          <form onSubmit={handleCreate} className="form">
            <div className="form-group">
              <label>Name</label>
              <input type="text" value={name} onChange={(e) => setName(e.target.value)} required />
            </div>
            <div className="form-group">
              <label>Type</label>
              <select
                value={typeId}
                onChange={(e) => setTypeId(e.target.value)}
                required
              >
                <option value="">Select type...</option>
                {credentialTypes?.map((t) => (
                  <option key={t.id} value={t.id}>
                    {t.name}
                  </option>
                ))}
              </select>
            </div>
            <div className="form-group">
              <label>Inputs (JSON)</label>
              <textarea
                value={inputs}
                onChange={(e) => setInputs(e.target.value)}
                placeholder={
                  selectedType
                    ? JSON.stringify(
                        Object.fromEntries(
                          Object.keys(selectedType.inputs_schema || {}).map((k) => [k, '']),
                        ),
                        null,
                        2,
                      )
                    : '{"username": "...", "password": "..."}'
                }
                rows={5}
              />
            </div>
            <div className="form-actions">
              <button type="submit" className="btn-primary" disabled={createCredential.isPending || !typeId}>
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
                <th>Type</th>
                <th>Created</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {credentials?.results?.map((cred) => {
                const typeName =
                  credentialTypes?.find((t) => t.id === cred.credential_type_id)?.name ??
                  cred.credential_type_id.slice(0, 8)
                return (
                  <tr key={cred.id}>
                    <td>{cred.name}</td>
                    <td>{typeName}</td>
                    <td>{new Date(cred.created_at).toLocaleDateString()}</td>
                    <td>
                      <button
                        className="btn-sm btn-danger"
                        onClick={() => {
                          if (confirm('Delete this credential?')) {
                            deleteCredential.mutate(cred.id)
                          }
                        }}
                      >
                        Delete
                      </button>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
