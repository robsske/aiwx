import { Link } from 'react-router-dom'
import { useProjects, useJobs, useJobTemplates } from '../hooks/useApi'
import { ErrorState } from '../components/ErrorState'
import { PageHeader, StatusBadge, EmptyState, LoadingState, Mono } from '../components/Page'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { jobDisplayName } from '../types'

export default function Dashboard() {
  const {
    data: projects,
    isLoading: projectsLoading,
    isError: projectsError,
    refetch: refetchProjects,
  } = useProjects()
  const {
    data: jobs,
    isLoading: jobsLoading,
    isError: jobsError,
    refetch: refetchJobs,
  } = useJobs()
  const { data: templates } = useJobTemplates()

  const recentJobs = jobs?.results?.slice(0, 5) ?? []
  const activeJobs =
    jobs?.results?.filter((j) => !['successful', 'failed', 'error', 'canceled'].includes(j.status))
      .length ?? 0
  const successfulJobs = jobs?.results?.filter((j) => j.status === 'successful').length ?? 0

  return (
    <div>
      <PageHeader title="Dashboard" />

      <div className="mb-6 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {[
          { value: projects?.results?.length ?? 0, label: 'Projects' },
          { value: jobs?.results?.length ?? 0, label: 'Total Jobs' },
          { value: activeJobs, label: 'Active Jobs' },
          { value: successfulJobs, label: 'Successful' },
        ].map((stat) => (
          <Card key={stat.label}>
            <CardContent className="pt-6">
              <div className="text-3xl font-semibold tracking-tight">{stat.value}</div>
              <div className="text-sm text-muted-foreground">{stat.label}</div>
            </CardContent>
          </Card>
        ))}
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Recent Jobs</CardTitle>
          </CardHeader>
          <CardContent>
            {jobsLoading && !jobs ? (
              <LoadingState />
            ) : jobsError && !jobs ? (
              <ErrorState message="Failed to load jobs" onRetry={() => refetchJobs()} />
            ) : recentJobs.length === 0 ? (
              <EmptyState message="No jobs yet" />
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Name</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>Created</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {recentJobs.map((job) => (
                    <TableRow key={job.id}>
                      <TableCell>
                        <Link
                          to={`/jobs/${job.id}`}
                          className="font-medium text-primary hover:underline"
                        >
                          {jobDisplayName(job, templates?.results)}
                        </Link>
                      </TableCell>
                      <TableCell><StatusBadge status={job.status} /></TableCell>
                      <TableCell>{new Date(job.created_at).toLocaleString()}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Recent Projects</CardTitle>
          </CardHeader>
          <CardContent>
            {projectsLoading && !projects ? (
              <LoadingState />
            ) : projectsError && !projects ? (
              <ErrorState message="Failed to load projects" onRetry={() => refetchProjects()} />
            ) : projects?.results?.length === 0 ? (
              <EmptyState message="No projects yet" />
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Name</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>Revision</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {projects?.results?.slice(0, 5).map((project) => (
                    <TableRow key={project.id}>
                      <TableCell>{project.name}</TableCell>
                      <TableCell><StatusBadge status={project.sync_status} /></TableCell>
                      <TableCell><Mono>{project.latest_revision?.slice(0, 8) ?? '—'}</Mono></TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
