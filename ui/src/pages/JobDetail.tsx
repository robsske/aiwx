import { useParams, useNavigate, Link } from 'react-router-dom'
import { useJob, useJobEvents, useJobLogs, useCancelJob } from '../hooks/useApi'
import { ErrorState } from '../components/ErrorState'
import { PageHeader, StatusBadge, EmptyState, LoadingState, Mono } from '../components/Page'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

function EventRow({ event }: { event: { id: number; event_type: string; event_data: Record<string, unknown>; created_at: string } }) {
  const data = event.event_data ?? {}
  return (
    <TableRow>
      <TableCell className="whitespace-nowrap text-muted-foreground">
        {new Date(event.created_at).toLocaleTimeString()}
      </TableCell>
      <TableCell>
        <Badge variant="outline">{event.event_type}</Badge>
      </TableCell>
      <TableCell>
        <div className="flex flex-wrap gap-x-4 gap-y-1">
          {Object.entries(data).map(([key, value]) => (
            <span key={key} className="text-xs">
              <strong>{key}:</strong> {String(value)}
            </span>
          ))}
        </div>
      </TableCell>
    </TableRow>
  )
}

function LogLine({ log }: { log: { sequence: number; content: string; source: string } }) {
  const sourceClass = log.source === 'stderr' ? 'log-failed' : log.source === 'system' ? 'log-changed' : 'log-ok'
  return (
    <div className={`log-line ${sourceClass}`}>
      <span>[{log.source}]</span>{' '}
      <span>{log.content}</span>
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
    return <LoadingState />
  }

  if (isError) {
    return <ErrorState message="Failed to load job" onRetry={() => refetch()} />
  }

  if (!job) {
    return <ErrorState message="Job not found" />
  }

  return (
    <div>
      <PageHeader
        title={`Job ${job.id.slice(0, 8)}`}
        actions={
          <>
            {!isTerminal && (
              <Button
                variant="destructive"
                onClick={() => cancelJob.mutate(job.id)}
                disabled={cancelJob.isPending}
              >
                Cancel Job
              </Button>
            )}
            <Button variant="secondary" onClick={() => navigate('/jobs')}>
              Back to Jobs
            </Button>
          </>
        }
      />

      <Card className="mb-6">
        <CardHeader>
          <CardTitle>Job Details</CardTitle>
        </CardHeader>
        <CardContent>
          <dl className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
            <div>
              <dt className="text-xs font-medium text-muted-foreground">Status</dt>
              <dd className="mt-1">
                <StatusBadge status={job.status} />
              </dd>
            </div>
            <div>
              <dt className="text-xs font-medium text-muted-foreground">Exit Code</dt>
              <dd className="mt-1">{job.exit_code ?? '—'}</dd>
            </div>
            <div>
              <dt className="text-xs font-medium text-muted-foreground">Template</dt>
              <dd className="mt-1">
                {job.job_template_id ? (
                  <Link
                    to={`/job-templates`}
                    className="font-mono text-xs text-primary hover:underline"
                  >
                    {job.job_template_id.slice(0, 8)}
                  </Link>
                ) : (
                  <span>—</span>
                )}
              </dd>
            </div>
            <div>
              <dt className="text-xs font-medium text-muted-foreground">Revision</dt>
              <dd className="mt-1">
                <Mono>{job.project_revision?.slice(0, 8) ?? '—'}</Mono>
              </dd>
            </div>
            <div>
              <dt className="text-xs font-medium text-muted-foreground">Started</dt>
              <dd className="mt-1">{job.started_at ? new Date(job.started_at).toLocaleString() : '—'}</dd>
            </div>
            <div>
              <dt className="text-xs font-medium text-muted-foreground">Finished</dt>
              <dd className="mt-1">{job.finished_at ? new Date(job.finished_at).toLocaleString() : '—'}</dd>
            </div>
          </dl>
        </CardContent>
      </Card>

      <Card className="mb-6">
        <CardHeader>
          <CardTitle>Job Output</CardTitle>
        </CardHeader>
        <CardContent>
          {logsError ? (
            <ErrorState message="Failed to load logs" />
          ) : !logs?.results?.length ? (
            <EmptyState message="No logs yet" />
          ) : (
            <div className="job-logs">
              {logs.results.map((log) => (
                <LogLine key={log.sequence} log={log} />
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Events</CardTitle>
        </CardHeader>
        <CardContent>
          {eventsError ? (
            <ErrorState message="Failed to load events" />
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Time</TableHead>
                  <TableHead>Type</TableHead>
                  <TableHead>Details</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {events?.results?.map((event) => (
                  <EventRow key={event.id} event={event} />
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
