import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import {
  useWorkflowTemplates,
  useCreateWorkflowTemplate,
  useDeleteWorkflowTemplate,
  useLaunchWorkflow,
  useWorkflowJobs,
} from '../hooks/useApi'
import { ErrorState } from '../components/ErrorState'
import { PageHeader, StatusBadge, EmptyState, LoadingState } from '../components/Page'
import { ConfirmDialog } from '../components/ConfirmDialog'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Badge } from '@/components/ui/badge'
import { ListToolbar, ListPagination, useListControls } from '@/hooks/useListControls'

export default function Workflows() {
  const navigate = useNavigate()
  const { data: workflows, isLoading, isError, refetch } = useWorkflowTemplates()
  const { data: workflowJobs } = useWorkflowJobs()
  const createWorkflow = useCreateWorkflowTemplate()
  const deleteWorkflow = useDeleteWorkflowTemplate()
  const launchWorkflow = useLaunchWorkflow()
  const [showCreate, setShowCreate] = useState(false)
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [formError, setFormError] = useState<string | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<string | null>(null)

  const list = useListControls(workflows?.results, {
    getText: (w) => `${w.name} ${w.description}`,
    pageSize: 10,
  })

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    setFormError(null)
    try {
      const wf = await createWorkflow.mutateAsync({
        name: name.trim(),
        description: description.trim(),
        nodes: [],
        links: [],
      })
      setShowCreate(false)
      setName('')
      setDescription('')
      navigate(`/workflows/${wf.id}`)
    } catch {
      setFormError('Failed to create workflow')
    }
  }

  return (
    <div>
      <PageHeader
        title="Workflows"
        actions={<Button onClick={() => setShowCreate(true)}>+ New Workflow</Button>}
      />

      {formError && <ErrorState message={formError} />}

      {showCreate && (
        <Card className="mb-6">
          <CardHeader>
            <CardTitle>Create Workflow</CardTitle>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleCreate} className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="wf-name">Name</Label>
                <Input id="wf-name" value={name} onChange={(e) => setName(e.target.value)} required />
              </div>
              <div className="space-y-2">
                <Label htmlFor="wf-desc">Description</Label>
                <Input
                  id="wf-desc"
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                />
              </div>
              <div className="flex gap-2">
                <Button type="submit" disabled={createWorkflow.isPending}>
                  {createWorkflow.isPending ? 'Creating...' : 'Create & Open Builder'}
                </Button>
                <Button type="button" variant="outline" onClick={() => setShowCreate(false)}>
                  Cancel
                </Button>
              </div>
            </form>
          </CardContent>
        </Card>
      )}

      {isLoading && !workflows ? (
        <LoadingState />
      ) : isError && !workflows ? (
        <ErrorState message="Failed to load workflows" onRetry={() => refetch()} />
      ) : (
        <>
          <ListToolbar
            query={list.query}
            onQueryChange={list.setQuery}
            placeholder="Search workflows..."
          />
          {!list.pageItems.length ? (
            <Card>
              <CardContent>
                <EmptyState
                  message={
                    list.query
                      ? 'No workflows match your search.'
                      : 'No workflows yet. Create one to open the visual builder.'
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
                      <TableHead>Nodes</TableHead>
                      <TableHead>Updated</TableHead>
                      <TableHead>Actions</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {list.pageItems.map((wf) => (
                      <TableRow key={wf.id}>
                        <TableCell>
                          <Link
                            to={`/workflows/${wf.id}`}
                            className="font-medium text-primary hover:underline"
                          >
                            {wf.name}
                          </Link>
                          {wf.description && (
                            <div className="text-xs text-muted-foreground">{wf.description}</div>
                          )}
                        </TableCell>
                        <TableCell>
                          <Badge variant="secondary">{wf.nodes?.length ?? 0}</Badge>
                        </TableCell>
                        <TableCell>{new Date(wf.updated_at).toLocaleString()}</TableCell>
                        <TableCell>
                          <div className="flex flex-wrap gap-2">
                            <Button asChild size="sm" variant="outline">
                              <Link to={`/workflows/${wf.id}`}>Builder</Link>
                            </Button>
                            <Button
                              size="sm"
                              disabled={launchWorkflow.isPending}
                              onClick={async () => {
                                try {
                                  await launchWorkflow.mutateAsync(wf.id)
                                } catch {
                                  setFormError('Failed to launch workflow')
                                }
                              }}
                            >
                              Launch
                            </Button>
                            <Button
                              size="sm"
                              variant="destructive"
                              onClick={() => setDeleteTarget(wf.id)}
                            >
                              Delete
                            </Button>
                          </div>
                        </TableCell>
                      </TableRow>
                    ))}
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

      {workflowJobs?.results?.length ? (
        <Card className="mt-8">
          <CardHeader>
            <CardTitle>Recent Workflow Runs</CardTitle>
          </CardHeader>
          <CardContent className="p-0">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Workflow</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Started</TableHead>
                  <TableHead>Finished</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {workflowJobs.results.slice(0, 8).map((job) => (
                  <TableRow key={job.id}>
                    <TableCell>{job.workflow_name}</TableCell>
                    <TableCell>
                      <StatusBadge
                        status={
                          job.status === 'success' || job.status === 'successful'
                            ? 'successful'
                            : job.status
                        }
                      />
                    </TableCell>
                    <TableCell>
                      {job.started_at ? new Date(job.started_at).toLocaleString() : '—'}
                    </TableCell>
                    <TableCell>
                      {job.finished_at ? new Date(job.finished_at).toLocaleString() : '—'}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      ) : null}

      <ConfirmDialog
        open={!!deleteTarget}
        title="Delete Workflow"
        message="Delete this workflow template? This cannot be undone."
        confirmLabel="Delete"
        danger
        onConfirm={async () => {
          if (deleteTarget) await deleteWorkflow.mutateAsync(deleteTarget)
          setDeleteTarget(null)
        }}
        onCancel={() => setDeleteTarget(null)}
      />
    </div>
  )
}
