import { useProjects, useJobs } from '../hooks/useApi'

export default function Dashboard() {
  const { data: projects, isLoading: projectsLoading } = useProjects()
  const { data: jobs, isLoading: jobsLoading } = useJobs()

  const recentJobs = jobs?.results?.slice(0, 5) ?? []
  const activeJobs = jobs?.results?.filter((j) => !['successful', 'failed', 'error', 'canceled'].includes(j.status)).length ?? 0

  return (
    <div className="page">
      <h1 className="page-title">Dashboard</h1>

      <div className="stats-grid">
        <div className="stat-card">
          <div className="stat-value">{projects?.results?.length ?? 0}</div>
          <div className="stat-label">Projects</div>
        </div>
        <div className="stat-card">
          <div className="stat-value">{jobs?.results?.length ?? 0}</div>
          <div className="stat-label">Total Jobs</div>
        </div>
        <div className="stat-card">
          <div className="stat-value">{activeJobs}</div>
          <div className="stat-label">Active Jobs</div>
        </div>
        <div className="stat-card">
          <div className="stat-value">
            {jobs?.results?.filter((j) => j.status === 'successful').length ?? 0}
          </div>
          <div className="stat-label">Successful</div>
        </div>
      </div>

      <div className="content-grid">
        <div className="card">
          <h2 className="card-title">Recent Jobs</h2>
          {jobsLoading ? (
            <div className="loading">Loading...</div>
          ) : recentJobs.length === 0 ? (
            <div className="empty-state">No jobs yet</div>
          ) : (
            <table className="table">
              <thead>
                <tr>
                  <th>ID</th>
                  <th>Status</th>
                  <th>Created</th>
                </tr>
              </thead>
              <tbody>
                {recentJobs.map((job) => (
                  <tr key={job.id}>
                    <td className="mono">{job.id.slice(0, 8)}</td>
                    <td>
                      <span className={`status-badge status-${job.status}`}>
                        {job.status}
                      </span>
                    </td>
                    <td>{new Date(job.created_at).toLocaleString()}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>

        <div className="card">
          <h2 className="card-title">Recent Projects</h2>
          {projectsLoading ? (
            <div className="loading">Loading...</div>
          ) : projects?.results?.length === 0 ? (
            <div className="empty-state">No projects yet</div>
          ) : (
            <table className="table">
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Status</th>
                  <th>Revision</th>
                </tr>
              </thead>
              <tbody>
                {projects?.results?.slice(0, 5).map((project) => (
                  <tr key={project.id}>
                    <td>{project.name}</td>
                    <td>
                      <span className={`status-badge status-${project.sync_status}`}>
                        {project.sync_status}
                      </span>
                    </td>
                    <td className="mono">{project.latest_revision?.slice(0, 8) ?? '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </div>
    </div>
  )
}
