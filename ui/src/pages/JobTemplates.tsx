import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useJobTemplates, useProjects, useInventories, useOrganizations, useLaunchJob, useCreateJobTemplate } from '../hooks/useApi'
import { ErrorState } from '../components/ErrorState'

function extractErrorMessage(err: unknown): string {
  if (err && typeof err === 'object' && 'response' in err) {
    const resp = (err as { response?: { data?: { message?: string } } }).response
    if (resp?.data?.message) return resp.data.message
  }
  if (err instanceof Error) return err.message
  return 'An unexpected error occurred'
}

export default function JobTemplates() {
  const navigate = useNavigate()
  const { data: templates, isLoading, isError, refetch } = useJobTemplates()
  const { data: projects } = useProjects()
  const { data: inventories } = useInventories()
  const { data: orgs } = useOrganizations()
  const launchJob = useLaunchJob()
  const createTemplate = useCreateJobTemplate()
  const [showCreate, setShowCreate] = useState(false)
  const [name, setName] = useState('')
  const [desc, setDesc] = useState('')
  const [projectId, setProjectId] = useState('')
  const [inventoryId, setInventoryId] = useState('')
  const [playbook, setPlaybook] = useState('')
  const [limit, setLimit] = useState('')
  const [verbosity, setVerbosity] = useState(0)
  const [forks, setForks] = useState(0)
  const [jobTags, setJobTags] = useState('')
  const [skipTags, setSkipTags] = useState('')
  const [formError, setFormError] = useState<string | null>(null)

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    setFormError(null)

    if (!name.trim()) return setFormError('Name is required')
    if (!projectId) return setFormError('Please select a project')
    if (!playbook.trim()) return setFormError('Playbook is required')

    try {
      await createTemplate.mutateAsync({
        name: name.trim(),
        description: desc.trim(),
        project_id: projectId,
        inventory_id: inventoryId || undefined,
        playbook: playbook.trim(),
        limit: limit.trim(),
        verbosity,
        forks,
        job_tags: jobTags.trim(),
        skip_tags: skipTags.trim(),
      })
      setShowCreate(false)
      resetForm()
    } catch (err: unknown) {
      setFormError(extractErrorMessage(err))
    }
  }

  const resetForm = () => {
    setName('')
    setDesc('')
    setProjectId('')
    setInventoryId('')
    setPlaybook('')
    setLimit('')
    setVerbosity(0)
    setForks(0)
    setJobTags('')
    setSkipTags('')
  }

  const handleLaunch = async (templateId: string) => {
    setFormError(null)
    try {
      const job = await launchJob.mutateAsync({ templateId })
      navigate(`/jobs/${job.id}`)
    } catch (err: unknown) {
      setFormError(extractErrorMessage(err))
    }
  }

  return (
    <div className="page">
      <div className="page-header">
        <h1 className="page-title">Job Templates</h1>
        <button className="btn-primary" onClick={() => setShowCreate(true)}>
          + New Template
        </button>
      </div>

      {formError && <ErrorState message={formError} />}

      {showCreate && (
        <div className="card">
          <h2 className="card-title">Create Job Template</h2>
          <form onSubmit={handleCreate} className="form">
            <div className="form-group">
              <label>Name</label>
              <input value={name} onChange={e => setName(e.target.value)} required />
            </div>
            <div className="form-group">
              <label>Description</label>
              <input value={desc} onChange={e => setDesc(e.target.value)} />
            </div>
            <div className="form-group">
              <label>Project *</label>
              <select value={projectId} onChange={e => setProjectId(e.target.value)} required>
                <option value="">Select project...</option>
                {projects?.results?.map(p => (
                  <option key={p.id} value={p.id}>{p.name}</option>
                ))}
              </select>
            </div>
            <div className="form-group">
              <label>Inventory</label>
              <select value={inventoryId} onChange={e => setInventoryId(e.target.value)}>
                <option value="">None</option>
                {inventories?.results?.map(i => (
                  <option key={i.id} value={i.id}>{i.name}</option>
                ))}
              </select>
            </div>
            <div className="form-group">
              <label>Playbook *</label>
              <input value={playbook} onChange={e => setPlaybook(e.target.value)} placeholder="site.yml" required />
            </div>

            <h3 style={{ margin: '1rem 0 0.5rem', fontSize: '0.875rem', color: 'var(--text-secondary)' }}>
              Host Targeting
            </h3>
            <div className="form-group">
              <label>Limit (host pattern)</label>
              <input
                value={limit}
                onChange={e => setLimit(e.target.value)}
                placeholder="web:db  or  hostname1:hostname2"
              />
              <small style={{ color: 'var(--text-secondary)', fontSize: '0.75rem' }}>
                Ansible --limit pattern. Use colons for OR, colons for AND (e.g., web:&staging)
              </small>
            </div>

            <h3 style={{ margin: '1rem 0 0.5rem', fontSize: '0.875rem', color: 'var(--text-secondary)' }}>
              Execution Options
            </h3>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: '1rem' }}>
              <div className="form-group">
                <label>Verbosity</label>
                <select value={verbosity} onChange={e => setVerbosity(Number(e.target.value))}>
                  <option value={0}>0 (Normal)</option>
                  <option value={1}>1 (-v)</option>
                  <option value={2}>2 (-vv)</option>
                  <option value={3}>3 (-vvv)</option>
                  <option value={4}>4 (-vvvv)</option>
                </select>
              </div>
              <div className="form-group">
                <label>Forks</label>
                <input
                  type="number"
                  min="0"
                  value={forks}
                  onChange={e => setForks(Number(e.target.value))}
                  placeholder="0 (default)"
                />
              </div>
              <div className="form-group">
                <label>Job Tags</label>
                <input
                  value={jobTags}
                  onChange={e => setJobTags(e.target.value)}
                  placeholder="install,config"
                />
              </div>
            </div>
            <div className="form-group">
              <label>Skip Tags</label>
              <input
                value={skipTags}
                onChange={e => setSkipTags(e.target.value)}
                placeholder="debug,tests"
              />
            </div>

            <div className="form-actions">
              <button type="submit" className="btn-primary" disabled={createTemplate.isPending}>
                {createTemplate.isPending ? 'Creating...' : 'Create'}
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
        <ErrorState message="Failed to load job templates" onRetry={() => refetch()} />
      ) : templates?.results?.length === 0 ? (
        <div className="empty-state">No job templates yet</div>
      ) : (
        <div className="card">
          <table className="table">
            <thead>
              <tr>
                <th>Name</th>
                <th>Organization</th>
                <th>Playbook</th>
                <th>Inventory</th>
                <th>Limit</th>
                <th>Verbosity</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {templates?.results?.map(jt => (
                <tr key={jt.id}>
                  <td>
                    <div style={{ fontWeight: 500 }}>{jt.name}</div>
                    {jt.description && (
                      <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>{jt.description}</div>
                    )}
                  </td>
                  <td>{orgs?.results?.find(o => o.id === jt.organization_id)?.name ?? jt.organization_id.slice(0, 8)}</td>
                  <td className="mono">{jt.playbook}</td>
                  <td>{jt.inventory_id ? 'Yes' : '—'}</td>
                  <td className="mono">{jt.limit || '—'}</td>
                  <td>{jt.verbosity > 0 ? `-${'v'.repeat(jt.verbosity)}` : 'Normal'}</td>
                  <td>
                    <button
                      className="btn-sm btn-primary"
                      onClick={() => handleLaunch(jt.id)}
                      disabled={launchJob.isPending}
                    >
                      Launch
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
