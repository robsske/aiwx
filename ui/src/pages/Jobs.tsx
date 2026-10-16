import { Link } from 'react-router-dom'
import { useJobs, useJobTemplates } from '../hooks/useApi'
import { ErrorState } from '../components/ErrorState'
import { PageHeader, StatusBadge, EmptyState, LoadingState } from '../components/Page'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { ListToolbar, ListPagination, useListControls } from '@/hooks/useListControls'
import { jobDisplayName } from '../types'

export default function Jobs() {
  const { data: jobs, isLoading, isError, error, refetch } = useJobs()
  const { data: templates } = useJobTemplates()
  const list = useListControls(jobs?.results, {
    getText: (job) =>
      `${jobDisplayName(job, templates?.results)} ${job.id} ${job.status} ${job.created_at} ${job.exit_code ?? ''}`,
    pageSize: 10,
  })

  return (
    <div>
      <PageHeader title="Jobs" />

      {isLoading && !jobs ? (
        <LoadingState />
      ) : isError && !jobs ? (
        <ErrorState
          message={error instanceof Error ? error.message : 'Failed to load jobs'}
          onRetry={() => refetch()}
        />
      ) : (
        <>
          <ListToolbar
            query={list.query}
            onQueryChange={list.setQuery}
            placeholder="Search by name, ID, status..."
          />

          {!list.pageItems.length ? (
            <Card>
              <CardContent>
                <EmptyState
                  message={
                    list.query
                      ? 'No jobs match your search.'
                      : 'No jobs yet. Launch a job from a Job Template.'
                  }
                />
              </CardContent>
            </Card>
          ) : (
            <Card>
              <CardContent className="p-0">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Name</TableHead>
                      <TableHead>ID</TableHead>
                      <TableHead>Status</TableHead>
                      <TableHead>Exit Code</TableHead>
                      <TableHead>Created</TableHead>
                      <TableHead>Duration</TableHead>
                      <TableHead>Actions</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {list.pageItems.map((job) => {
                      const name = jobDisplayName(job, templates?.results)
                      return (
                        <TableRow key={job.id}>
                          <TableCell>
                            <Link
                              to={`/jobs/${job.id}`}
                              className="font-medium text-primary hover:underline"
                            >
                              {name}
                            </Link>
                          </TableCell>
                          <TableCell>
                            <Link
                              to={`/jobs/${job.id}`}
                              className="font-mono text-xs text-muted-foreground hover:underline"
                            >
                              {job.id.slice(0, 8)}
                            </Link>
                          </TableCell>
                          <TableCell><StatusBadge status={job.status} /></TableCell>
                          <TableCell>{job.exit_code ?? '—'}</TableCell>
                          <TableCell>{new Date(job.created_at).toLocaleString()}</TableCell>
                          <TableCell>
                            {job.started_at && job.finished_at
                              ? `${Math.round(
                                  (new Date(job.finished_at).getTime() -
                                    new Date(job.started_at).getTime()) /
                                    1000,
                                )}s`
                              : job.started_at
                                ? 'Running...'
                                : '—'}
                          </TableCell>
                          <TableCell>
                            <Button asChild variant="outline" size="sm">
                              <Link to={`/jobs/${job.id}`}>View</Link>
                            </Button>
                          </TableCell>
                        </TableRow>
                      )
                    })}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>
          )}

          <ListPagination
            page={list.page}
            totalPages={list.totalPages}
            pageSize={list.pageSize}
            total={list.total}
            rangeLabel={list.rangeLabel}
            onPageChange={list.setPage}
            onPageSizeChange={list.setPageSize}
          />
        </>
      )}
    </div>
  )
}
