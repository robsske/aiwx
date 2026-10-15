import { useState } from 'react'
import { Link } from 'react-router-dom'
import { useProjects, useCreateProject, useSyncProject } from '../hooks/useApi'
import { ErrorState } from '../components/ErrorState'

function extractErrorMessage(err: unknown): string {
  if (err && typeof err === 'object' && 'response' in err) {
    const resp = (err as { response?: { data?: { message?: string } } }).response
    if (resp?.data?.message) return resp.data.message
  }
  if (err instanceof Error) return err.message
  return 'An unexpected error occurred'
}

export default function Projects() {
  const { data: projects, isLoading, isError, refetch } = useProjects()
  const createProject = useCreateProject()
  const syncProject = useSyncProject()
  const [showCreate, setShowCreate] = useState(false)
  const [name, setName] = useState('')
  const [scmUrl, setScmUrl] = useState('')
  const [scmBranch, setScmBranch] = useState('main')
  const [formError, setFormError] = useState<string | null>(null)
  const [syncingId, setSyncingId] = useState<string | null>(null)

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    setFormError(null)
    try {
      await createProject.mutateAsync({
        name: name.trim(),
        scm_url: scmUrl.trim(),
        scm_branch: scmBranch.trim() || 'main',
      })
      setShowCreate(false)
      setName('')
      setScmUrl('')
      setScmBranch('main')
    } catch (err: unknown) {
      setFormError(extractErrorMessage(err))
    }
  }

  const handleSync = async (projectId: string) => {
    setFormError(null)
    setSyncingId(projectId)
    try {
      await syncProject.mutateAsync(projectId)
      // Force refetch to get updated status
      setTimeout(() => refetch(), 500)
    } catch (err: unknown) {
      setFormError(extractErrorMessage(err))
    } finally {
      setSyncingId(null)
    }
  }

  return (
    <div className="page">
      <div className="page-header">
        <h1 className="page-title">Projects</h1>
        <button className="btn-primary" onClick={() => setShowCreate(true)}>
          + New Project
        </button>
      </div>

      {formError && <ErrorState message={formError} />}

      {showCreate && (
        <div className="card">
          <h2 className="card-title">Create Project</h2>
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
              <label>Git URL</label>
              <input
                type="url"
                value={scmUrl}
                onChange={(e) => setScmUrl(e.target.value)}
                placeholder="https://github.com/org/repo.git"
                required
              />
            </div>
            <div className="form-group">
              <label>Branch</label>
              <input
                type="text"
                value={scmBranch}
                onChange={(e) => setScmBranch(e.target.value)}
                placeholder="main"
              />
            </div>
            <div className="form-actions">
              <button type="submit" className="btn-primary" disabled={createProject.isPending}>
                {createProject.isPending ? 'Creating...' : 'Create'}
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
        <ErrorState message="Failed to load projects" onRetry={() => refetch()} />
      ) : (
        <div className="card">
          <table className="table">
            <thead>
              <tr>
                <th>Name</th>
                <th>URL</th>
                <th>Branch</th>
                <th>Status</th>
                <th>Revision</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {projects?.results?.map((project) => (
                <tr key={project.id}>
                  <td>
                    <Link to={`/projects/${project.id}`} className="link">
                      {project.name}
                    </Link>
                  </td>
                  <td className="mono">{project.scm_url}</td>
                  <td>{project.scm_branch}</td>
                  <td>
                    <span className={`status-badge status-${project.sync_status}`}>
                      {project.sync_status}
                    </span>
                  </td>
                  <td className="mono">{project.latest_revision?.slice(0, 8) ?? '—'}</td>
                  <td>
                    <button
                      className="btn-sm"
                      onClick={() => handleSync(project.id)}
                      disabled={syncingId === project.id || project.sync_status === 'running'}
                    >
                      {syncingId === project.id ? 'Syncing...' : 'Sync'}
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
