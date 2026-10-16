import { useState } from 'react'
import { Link } from 'react-router-dom'
import { useProjects, useOrganizations, useCreateProject, useSyncProject } from '../hooks/useApi'
import { ErrorState } from '../components/ErrorState'
import { PageHeader, StatusBadge, LoadingState, Mono, EmptyState } from '../components/Page'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { ListToolbar, ListPagination, useListControls } from '@/hooks/useListControls'

function extractErrorMessage(err: unknown): string {
  if (err && typeof err === 'object' && 'response' in err) {
    const resp = (err as { response?: { data?: { message?: string } } }).response
    if (resp?.data?.message) return resp.data.message
  }
  if (err instanceof Error) return err.message
  return 'An unexpected error occurred'
}

export default function Projects() {
  const { data: projects, isLoading, isError, refetch } = useProjects()
  const { data: orgs } = useOrganizations()
  const createProject = useCreateProject()
  const syncProject = useSyncProject()
  const [showCreate, setShowCreate] = useState(false)
  const [orgId, setOrgId] = useState('')
  const [name, setName] = useState('')
  const [scmUrl, setScmUrl] = useState('')
  const [scmBranch, setScmBranch] = useState('main')
  const [formError, setFormError] = useState<string | null>(null)
  const [syncingId, setSyncingId] = useState<string | null>(null)

  const orgName = (id: string) => orgs?.results?.find(o => o.id === id)?.name ?? id.slice(0, 8)

  const list = useListControls(projects?.results, {
    getText: (p) => `${p.name} ${p.scm_url} ${p.scm_branch} ${p.sync_status} ${orgName(p.organization_id)}`,
    pageSize: 10,
  })

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    setFormError(null)
    try {
      await createProject.mutateAsync({
        organization_id: orgId || undefined,
        name: name.trim(),
        scm_url: scmUrl.trim(),
        scm_branch: scmBranch.trim() || 'main',
      })
      setShowCreate(false)
      setOrgId('')
      setName('')
      setScmUrl('')
      setScmBranch('main')
    } catch (err: unknown) {
      setFormError(extractErrorMessage(err))
    }
  }

  const handleSync = async (projectId: string) => {
    setFormError(null)
    setSyncingId(projectId)
    try {
      await syncProject.mutateAsync(projectId)
      setTimeout(() => refetch(), 500)
    } catch (err: unknown) {
      setFormError(extractErrorMessage(err))
    } finally {
      setSyncingId(null)
    }
  }

  return (
    <div>
      <PageHeader
        title="Projects"
        actions={<Button onClick={() => setShowCreate(true)}>+ New Project</Button>}
      />

      {formError && <ErrorState message={formError} />}

      {showCreate && (
        <Card className="mb-6">
          <CardHeader>
            <CardTitle>Create Project</CardTitle>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleCreate} className="space-y-4">
              <div className="space-y-2">
                <Label>Organization</Label>
                <Select value={orgId || '__default__'} onValueChange={(v: string) => setOrgId(v === '__default__' ? '' : v)}>
                  <SelectTrigger>
                    <SelectValue placeholder="Default (from auth)" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="__default__">Default (from auth)</SelectItem>
                    {orgs?.results?.map((o) => (
                      <SelectItem key={o.id} value={o.id}>{o.name}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label>Name</Label>
                <Input value={name} onChange={(e) => setName(e.target.value)} required />
              </div>
              <div className="space-y-2">
                <Label>Git URL</Label>
                <Input
                  type="url"
                  value={scmUrl}
                  onChange={(e) => setScmUrl(e.target.value)}
                  placeholder="https://github.com/org/repo.git"
                  required
                />
              </div>
              <div className="space-y-2">
                <Label>Branch</Label>
                <Input value={scmBranch} onChange={(e) => setScmBranch(e.target.value)} placeholder="main" />
              </div>
              <div className="flex gap-2">
                <Button type="submit" disabled={createProject.isPending}>
                  {createProject.isPending ? 'Creating...' : 'Create'}
                </Button>
                <Button type="button" variant="outline" onClick={() => setShowCreate(false)}>
                  Cancel
                </Button>
              </div>
            </form>
          </CardContent>
        </Card>
      )}

      {isLoading && !projects ? (
        <LoadingState />
      ) : isError && !projects ? (
        <ErrorState message="Failed to load projects" onRetry={() => refetch()} />
      ) : (
        <>
          <ListToolbar
            query={list.query}
            onQueryChange={list.setQuery}
            placeholder="Search projects..."
          />
          {!list.pageItems.length ? (
            <Card>
              <CardContent>
                <EmptyState message={list.query ? 'No projects match your search.' : 'No projects yet.'} />
              </CardContent>
            </Card>
          ) : (
            <Card>
              <CardContent className="p-0">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Name</TableHead>
                      <TableHead>Organization</TableHead>
                      <TableHead>URL</TableHead>
                      <TableHead>Branch</TableHead>
                      <TableHead>Status</TableHead>
                      <TableHead>Revision</TableHead>
                      <TableHead>Actions</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {list.pageItems.map((project) => (
                      <TableRow key={project.id}>
                        <TableCell>
                          <Link to={`/projects/${project.id}`} className="text-primary hover:underline">
                            {project.name}
                          </Link>
                        </TableCell>
                        <TableCell>{orgName(project.organization_id)}</TableCell>
                        <TableCell><Mono>{project.scm_url}</Mono></TableCell>
                        <TableCell>{project.scm_branch}</TableCell>
                        <TableCell>
                          <StatusBadge status={project.sync_status} />
                        </TableCell>
                        <TableCell>
                          <Mono>{project.latest_revision?.slice(0, 8) ?? '—'}</Mono>
                        </TableCell>
                        <TableCell>
                          <Button
                            size="sm"
                            variant="outline"
                            onClick={() => handleSync(project.id)}
                            disabled={syncingId === project.id || project.sync_status === 'running'}
                          >
                            {syncingId === project.id ? 'Syncing...' : 'Sync'}
                          </Button>
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
    </div>
  )
}
