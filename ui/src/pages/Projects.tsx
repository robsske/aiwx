import { useState } from 'react'
import { Link } from 'react-router-dom'
import { useProjects, useCreateProject, useSyncProject } from '../hooks/useApi'

export default function Projects() {
  const { data: projects, isLoading } = useProjects()
  const createProject = useCreateProject()
  const syncProject = useSyncProject()
  const [showCreate, setShowCreate] = useState(false)
  const [name, setName] = useState('')
  const [scmUrl, setScmUrl] = useState('')
  const [scmBranch, setScmBranch] = useState('main')

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    await createProject.mutateAsync({
      name,
      scm_url: scmUrl,
      scm_branch: scmBranch,
    })
    setShowCreate(false)
    setName('')
    setScmUrl('')
    setScmBranch('main')
  }

  return (
    <div className="page">
      <div className="page-header">
        <h1 className="page-title">Projects</h1>
        <button className="btn-primary" onClick={() => setShowCreate(true)}>
          + New Project
        </button>
      </div>

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
                required
              />
            </div>
            <div className="form-group">
              <label>Branch</label>
              <input
                type="text"
                value={scmBranch}
                onChange={(e) => setScmBranch(e.target.value)}
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
                      onClick={() => syncProject.mutate(project.id)}
                      disabled={syncProject.isPending || project.sync_status === 'running'}
                    >
                      Sync
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
