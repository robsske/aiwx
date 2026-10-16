import { useState } from 'react'
import { useCredentials, useCredentialTypes, useOrganizations, useCreateCredential, useDeleteCredential } from '../hooks/useApi'
import { ErrorState } from '../components/ErrorState'
import { ConfirmDialog } from '../components/ConfirmDialog'

function extractErrorMessage(err: unknown): string {
  if (err && typeof err === 'object' && 'response' in err) {
    const resp = (err as { response?: { data?: { message?: string } } }).response
    if (resp?.data?.message) return resp.data.message
  }
  if (err instanceof Error) return err.message
  return 'An unexpected error occurred'
}

export default function Credentials() {
  const { data: credentials, isLoading, isError, error, refetch } = useCredentials()
  const { data: credentialTypes, isLoading: typesLoading } = useCredentialTypes()
  const { data: orgs } = useOrganizations()
  const createCredential = useCreateCredential()
  const deleteCredential = useDeleteCredential()
  const [showCreate, setShowCreate] = useState(false)
  const [name, setName] = useState('')
  const [typeId, setTypeId] = useState('')
  const [inputs, setInputs] = useState('')
  const [formError, setFormError] = useState<string | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<string | null>(null)

  const selectedType = credentialTypes?.find((t) => t.id === typeId)

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    setFormError(null)

    if (!typeId) {
      setFormError('Please select a credential type')
      return
    }
    if (!name.trim()) {
      setFormError('Name is required')
      return
    }

    let parsedInputs: Record<string, unknown>
    try {
      parsedInputs = inputs.trim() ? JSON.parse(inputs) : {}
    } catch {
      setFormError('Invalid JSON for inputs')
      return
    }

    try {
      await createCredential.mutateAsync({
        name: name.trim(),
        credential_type_id: typeId,
        inputs: parsedInputs,
      })
      setShowCreate(false)
      setName('')
      setTypeId('')
      setInputs('')
    } catch (err: unknown) {
      setFormError(extractErrorMessage(err))
    }
  }

  const handleDelete = () => {
    if (deleteTarget) {
      deleteCredential.mutate(deleteTarget)
      setDeleteTarget(null)
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

      {formError && <ErrorState message={formError} />}

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
              {typesLoading ? (
                <div className="loading">Loading types...</div>
              ) : (
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
              )}
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
              <button
                type="submit"
                className="btn-primary"
                disabled={createCredential.isPending || !typeId || typesLoading}
              >
                {createCredential.isPending ? 'Creating...' : 'Create'}
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
      ) : isError ? (
        <ErrorState
          message={error instanceof Error ? error.message : 'Failed to load credentials'}
          onRetry={() => refetch()}
        />
      ) : (
        <div className="card">
          <table className="table">
            <thead>
              <tr>
                <th>Name</th>
                <th>Organization</th>
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
                    <td>{orgs?.results?.find(o => o.id === cred.organization_id)?.name ?? cred.organization_id.slice(0, 8)}</td>
                    <td>{typeName}</td>
                    <td>{new Date(cred.created_at).toLocaleDateString()}</td>
                    <td>
                      <button
                        className="btn-sm btn-danger"
                        onClick={() => setDeleteTarget(cred.id)}
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

      <ConfirmDialog
        open={!!deleteTarget}
        title="Delete Credential"
        message="Are you sure you want to delete this credential? This action cannot be undone."
        confirmLabel="Delete"
        danger
        onConfirm={handleDelete}
        onCancel={() => setDeleteTarget(null)}
      />
    </div>
  )
}
