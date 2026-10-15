import { useParams, useNavigate } from 'react-router-dom'
import { useJob, useJobEvents, useCancelJob } from '../hooks/useApi'

export default function JobDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { data: job, isLoading } = useJob(id!)
  const { data: events } = useJobEvents(id!)
  const cancelJob = useCancelJob()

  const isTerminal = job && ['successful', 'failed', 'error', 'canceled'].includes(job.status)

  if (isLoading) {
    return <div className="loading">Loading...</div>
  }

  if (!job) {
    return <div className="error">Job not found</div>
  }

  return (
    <div className="page">
      <div className="page-header">
        <h1 className="page-title">Job {job.id.slice(0, 8)}</h1>
        <div className="page-actions">
          {!isTerminal && (
            <button
              className="btn-danger"
              onClick={() => cancelJob.mutate(job.id)}
              disabled={cancelJob.isPending}
            >
              Cancel Job
            </button>
          )}
          <button className="btn-secondary" onClick={() => navigate('/jobs')}>
            Back to Jobs
          </button>
        </div>
      </div>

      <div className="content-grid">
        <div className="card">
          <h2 className="card-title">Job Details</h2>
          <div className="detail-grid">
            <div className="detail-item">
              <span className="detail-label">Status</span>
              <span className={`status-badge status-${job.status}`}>{job.status}</span>
            </div>
            <div className="detail-item">
              <span className="detail-label">Exit Code</span>
              <span>{job.exit_code ?? '—'}</span>
            </div>
            <div className="detail-item">
              <span className="detail-label">Revision</span>
              <span className="mono">{job.project_revision?.slice(0, 8) ?? '—'}</span>
            </div>
            <div className="detail-item">
              <span className="detail-label">K8s Job</span>
              <span className="mono">{job.kubernetes_job_name ?? '—'}</span>
            </div>
            <div className="detail-item">
              <span className="detail-label">Started</span>
              <span>{job.started_at ? new Date(job.started_at).toLocaleString() : '—'}</span>
            </div>
            <div className="detail-item">
              <span className="detail-label">Finished</span>
              <span>{job.finished_at ? new Date(job.finished_at).toLocaleString() : '—'}</span>
            </div>
          </div>
        </div>

        <div className="card">
          <h2 className="card-title">Events</h2>
          <div className="events-list">
            {events?.results?.map((event) => (
              <div key={event.id} className="event-item">
                <span className="event-time">
                  {new Date(event.created_at).toLocaleTimeString()}
                </span>
                <span className={`event-type event-${event.event_type.replace('.', '-')}`}>
                  {event.event_type}
                </span>
                <span className="event-data">
                  {JSON.stringify(event.event_data)}
                </span>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  )
}
