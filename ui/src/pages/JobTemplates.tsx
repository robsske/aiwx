import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useJobTemplates, useProjects, useLaunchJob, useCreateJobTemplate } from '../hooks/useApi'
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
  const {
    data: templates,
    isLoading,
    isError,
    refetch,
  } = useJobTemplates()
  const { data: projects } = useProjects()
  const launchJob = useLaunchJob()
  const createTemplate = useCreateJobTemplate()
  const [showCreate, setShowCreate] = useState(false)
  const [name, setName] = useState('')
  const [projectId, setProjectId] = useState('')
  const [playbook, setPlaybook] = useState('')
  const [formError, setFormError] = useState<string | null>(null)

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    setFormError(null)

    if (!name.trim()) {
      setFormError('Name is required')
      return
    }
    if (!projectId) {
      setFormError('Please select a project')
      return
    }
    if (!playbook.trim()) {
      setFormError('Playbook is required')
      return
    }

    try {
      await createTemplate.mutateAsync({
        name: name.trim(),
        project_id: projectId,
        playbook: playbook.trim(),
      })
      setShowCreate(false)
      setName('')
      setProjectId('')
      setPlaybook('')
    } catch (err: unknown) {
      setFormError(extractErrorMessage(err))
    }
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
              <input
                type="text"
                value={name}
                onChange={(e) => setName(e.target.value)}
                required
              />
            </div>
            <div className="form-group">
              <label>Project</label>
              <select
                value={projectId}
                onChange={(e) => setProjectId(e.target.value)}
                required
              >
                <option value="">Select project...</option>
                {projects?.results?.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.name}
                  </option>
                ))}
              </select>
            </div>
            <div className="form-group">
              <label>Playbook</label>
              <input
                type="text"
                value={playbook}
                onChange={(e) => setPlaybook(e.target.value)}
                placeholder="site.yml"
                required
              />
            </div>
            <div className="form-actions">
              <button type="submit" className="btn-primary" disabled={createTemplate.isPending}>
                Create
              </button>
              <button
                type="button"
                className="btn-secondary"
                onClick={() => setShowCreate(false)}
              >
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
                <th>Playbook</th>
                <th>Created</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {templates?.results?.map((jt) => (
                <tr key={jt.id}>
                  <td>{jt.name}</td>
                  <td className="mono">{jt.playbook}</td>
                  <td>{new Date(jt.created_at).toLocaleDateString()}</td>
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
