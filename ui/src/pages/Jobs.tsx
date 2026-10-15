import { Link } from 'react-router-dom'
import { useJobs } from '../hooks/useApi'
import { ErrorState } from '../components/ErrorState'

export default function Jobs() {
  const { data: jobs, isLoading, isError, error, refetch } = useJobs()

  return (
    <div className="page">
      <h1 className="page-title">Jobs</h1>

      {isLoading && !jobs ? (
        <div className="loading">Loading...</div>
      ) : isError && !jobs ? (
        <ErrorState
          message={error instanceof Error ? error.message : 'Failed to load jobs'}
          onRetry={() => refetch()}
        />
      ) : !jobs?.results?.length ? (
        <div className="card">
          <div className="empty-state">No jobs yet. Launch a job from a Job Template.</div>
        </div>
      ) : (
        <div className="card">
          <table className="table">
            <thead>
              <tr>
                <th>ID</th>
                <th>Status</th>
                <th>Exit Code</th>
                <th>Created</th>
                <th>Duration</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {jobs.results.map((job) => (
                <tr key={job.id}>
                  <td>
                    <Link to={`/jobs/${job.id}`} className="link mono">
                      {job.id.slice(0, 8)}
                    </Link>
                  </td>
                  <td>
                    <span className={`status-badge status-${job.status}`}>
                      {job.status}
                    </span>
                  </td>
                  <td>{job.exit_code ?? '—'}</td>
                  <td>{new Date(job.created_at).toLocaleString()}</td>
                  <td>
                    {job.started_at && job.finished_at
                      ? `${Math.round(
                          (new Date(job.finished_at).getTime() -
                            new Date(job.started_at).getTime()) /
                            1000,
                        )}s`
                      : job.started_at
                        ? 'Running...'
                        : '—'}
                  </td>
                  <td>
                    <Link to={`/jobs/${job.id}`} className="btn-sm">
                      View
                    </Link>
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
