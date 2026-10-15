import { useParams } from 'react-router-dom'
import { useProject } from '../hooks/useApi'

export default function ProjectDetail() {
  const { id } = useParams<{ id: string }>()
  const { data: project, isLoading } = useProject(id!)

  if (isLoading) {
    return <div className="loading">Loading...</div>
  }

  if (!project) {
    return <div className="error">Project not found</div>
  }

  return (
    <div className="page">
      <h1 className="page-title">{project.name}</h1>

      <div className="card">
        <h2 className="card-title">Project Details</h2>
        <div className="detail-grid">
          <div className="detail-item">
            <span className="detail-label">Git URL</span>
            <span className="mono">{project.scm_url}</span>
          </div>
          <div className="detail-item">
            <span className="detail-label">Branch</span>
            <span>{project.scm_branch}</span>
          </div>
          <div className="detail-item">
            <span className="detail-label">Sync Status</span>
            <span className={`status-badge status-${project.sync_status}`}>
              {project.sync_status}
            </span>
          </div>
          <div className="detail-item">
            <span className="detail-label">Latest Revision</span>
            <span className="mono">{project.latest_revision?.slice(0, 8) ?? '—'}</span>
          </div>
          <div className="detail-item">
            <span className="detail-label">Last Sync</span>
            <span>{project.last_sync_at ? new Date(project.last_sync_at).toLocaleString() : '—'}</span>
          </div>
          <div className="detail-item">
            <span className="detail-label">Created</span>
            <span>{new Date(project.created_at).toLocaleString()}</span>
          </div>
        </div>
      </div>
    </div>
  )
}
