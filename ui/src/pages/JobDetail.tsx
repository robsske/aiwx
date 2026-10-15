import { useParams, useNavigate, Link } from 'react-router-dom'
import { useJob, useJobEvents, useJobLogs, useCancelJob } from '../hooks/useApi'
import { ErrorState } from '../components/ErrorState'

function EventRow({ event }: { event: { id: number; event_type: string; event_data: Record<string, unknown>; created_at: string } }) {
  const data = event.event_data ?? {}
  return (
    <div className="event-item">
      <span className="event-time">{new Date(event.created_at).toLocaleTimeString()}</span>
      <span className={`event-type event-${event.event_type.replace('.', '-')}`}>
        {event.event_type}
      </span>
      <div className="event-details">
        {Object.entries(data).map(([key, value]) => (
          <span key={key} className="event-field">
            <strong>{key}:</strong> {String(value)}
          </span>
        ))}
      </div>
    </div>
  )
}

function LogLine({ log }: { log: { sequence: number; content: string; source: string } }) {
  const sourceClass = log.source === 'stderr' ? 'log-stderr' : log.source === 'system' ? 'log-system' : 'log-stdout'
  return (
    <div className={`log-line ${sourceClass}`}>
      <span className="log-source">[{log.source}]</span>
      <span className="log-content">{log.content}</span>
    </div>
  )
}

export default function JobDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { data: job, isLoading, isError, refetch } = useJob(id!)
  const { data: events, isError: eventsError } = useJobEvents(id!)
  const { data: logs, isError: logsError } = useJobLogs(id!)
  const cancelJob = useCancelJob()

  const isTerminal = job && ['successful', 'failed', 'error', 'canceled'].includes(job.status)

  if (isLoading) {
    return <div className="loading">Loading...</div>
  }

  if (isError) {
    return <ErrorState message="Failed to load job" onRetry={() => refetch()} />
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
            <span className="detail-label">Template</span>
            {job.job_template_id ? (
              <Link to={`/job-templates`} className="mono">
                {job.job_template_id.slice(0, 8)}
              </Link>
            ) : (
              <span>—</span>
            )}
          </div>
          <div className="detail-item">
            <span className="detail-label">Revision</span>
            <span className="mono">{job.project_revision?.slice(0, 8) ?? '—'}</span>
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
        <h2 className="card-title">Job Output</h2>
        {logsError ? (
          <ErrorState message="Failed to load logs" />
        ) : !logs?.results?.length ? (
          <div className="empty-state">No logs yet</div>
        ) : (
          <div className="job-logs">
            {logs.results.map((log) => (
              <LogLine key={log.sequence} log={log} />
            ))}
          </div>
        )}
      </div>

      <div className="card">
        <h2 className="card-title">Events</h2>
        {eventsError ? (
          <ErrorState message="Failed to load events" />
        ) : (
          <div className="events-list">
            {events?.results?.map((event) => (
              <EventRow key={event.id} event={event} />
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
