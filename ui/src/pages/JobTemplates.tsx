import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useJobTemplates, useProjects, useInventories, useOrganizations, useLaunchJob, useCreateJobTemplate } from '../hooks/useApi'
import { ErrorState } from '../components/ErrorState'
import { PageHeader, EmptyState, LoadingState, Mono } from '../components/Page'
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

export default function JobTemplates() {
  const navigate = useNavigate()
  const { data: templates, isLoading, isError, refetch } = useJobTemplates()
  const { data: projects } = useProjects()
  const { data: inventories } = useInventories()
  const { data: orgs } = useOrganizations()
  const launchJob = useLaunchJob()
  const createTemplate = useCreateJobTemplate()
  const list = useListControls(templates?.results, {
    getText: (jt) =>
      `${jt.name} ${jt.description ?? ''} ${jt.playbook} ${jt.limit ?? ''} ${
        orgs?.results?.find((o) => o.id === jt.organization_id)?.name ?? ''
      }`,
    pageSize: 10,
  })
  const [showCreate, setShowCreate] = useState(false)
  const [name, setName] = useState('')
  const [desc, setDesc] = useState('')
  const [projectId, setProjectId] = useState('')
  const [inventoryId, setInventoryId] = useState('')
  const [playbook, setPlaybook] = useState('')
  const [limit, setLimit] = useState('')
  const [verbosity, setVerbosity] = useState(0)
  const [forks, setForks] = useState(0)
  const [jobTags, setJobTags] = useState('')
  const [skipTags, setSkipTags] = useState('')
  const [formError, setFormError] = useState<string | null>(null)

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    setFormError(null)

    if (!name.trim()) return setFormError('Name is required')
    if (!projectId) return setFormError('Please select a project')
    if (!playbook.trim()) return setFormError('Playbook is required')

    try {
      await createTemplate.mutateAsync({
        name: name.trim(),
        description: desc.trim(),
        project_id: projectId,
        inventory_id: inventoryId || undefined,
        playbook: playbook.trim(),
        limit: limit.trim(),
        verbosity,
        forks,
        job_tags: jobTags.trim(),
        skip_tags: skipTags.trim(),
      })
      setShowCreate(false)
      resetForm()
    } catch (err: unknown) {
      setFormError(extractErrorMessage(err))
    }
  }

  const resetForm = () => {
    setName('')
    setDesc('')
    setProjectId('')
    setInventoryId('')
    setPlaybook('')
    setLimit('')
    setVerbosity(0)
    setForks(0)
    setJobTags('')
    setSkipTags('')
  }

  const handleLaunch = async (templateId: string) => {
    setFormError(null)
    try {
      const job = await launchJob.mutateAsync({ templateId })
      navigate(`/jobs/${job.id}`)
    } catch (err: unknown) {
      setFormError(extractErrorMessage(err))
    }
  }

  return (
    <div>
      <PageHeader
        title="Job Templates"
        actions={<Button onClick={() => setShowCreate(true)}>+ New Template</Button>}
      />

      {formError && <ErrorState message={formError} />}

      {showCreate && (
        <Card className="mb-6">
          <CardHeader>
            <CardTitle>Create Job Template</CardTitle>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleCreate} className="space-y-4">
              <div className="space-y-2">
                <Label>Name</Label>
                <Input value={name} onChange={e => setName(e.target.value)} required />
              </div>
              <div className="space-y-2">
                <Label>Description</Label>
                <Input value={desc} onChange={e => setDesc(e.target.value)} />
              </div>
              <div className="grid gap-4 sm:grid-cols-2">
                <div className="space-y-2">
                  <Label>Project *</Label>
                  <Select value={projectId || undefined} onValueChange={setProjectId}>
                    <SelectTrigger>
                      <SelectValue placeholder="Select project..." />
                    </SelectTrigger>
                    <SelectContent>
                      {projects?.results?.map(p => (
                        <SelectItem key={p.id} value={p.id}>{p.name}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-2">
                  <Label>Inventory</Label>
                  <Select value={inventoryId || '__none__'} onValueChange={(v: string) => setInventoryId(v === '__none__' ? '' : v)}>
                    <SelectTrigger>
                      <SelectValue placeholder="None" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="__none__">None</SelectItem>
                      {inventories?.results?.map(i => (
                        <SelectItem key={i.id} value={i.id}>{i.name}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              </div>
              <div className="space-y-2">
                <Label>Playbook *</Label>
                <Input value={playbook} onChange={e => setPlaybook(e.target.value)} placeholder="site.yml" required />
              </div>

              <div>
                <h3 className="mb-2 text-sm font-medium text-muted-foreground">Host Targeting</h3>
                <div className="space-y-2">
                  <Label>Limit (host pattern)</Label>
                  <Input
                    value={limit}
                    onChange={e => setLimit(e.target.value)}
                    placeholder="web:db  or  hostname1:hostname2"
                  />
                  <p className="text-xs text-muted-foreground">
                    Ansible --limit pattern. Use colons for OR, colons for AND (e.g., web:&staging)
                  </p>
                </div>
              </div>

              <div>
                <h3 className="mb-2 text-sm font-medium text-muted-foreground">Execution Options</h3>
                <div className="grid gap-4 sm:grid-cols-3">
                  <div className="space-y-2">
                    <Label>Verbosity</Label>
                    <Select value={String(verbosity)} onValueChange={(v: string) => setVerbosity(Number(v))}>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="0">0 (Normal)</SelectItem>
                        <SelectItem value="1">1 (-v)</SelectItem>
                        <SelectItem value="2">2 (-vv)</SelectItem>
                        <SelectItem value="3">3 (-vvv)</SelectItem>
                        <SelectItem value="4">4 (-vvvv)</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="space-y-2">
                    <Label>Forks</Label>
                    <Input
                      type="number"
                      min={0}
                      value={forks}
                      onChange={e => setForks(Number(e.target.value))}
                      placeholder="0 (default)"
                    />
                  </div>
                  <div className="space-y-2">
                    <Label>Job Tags</Label>
                    <Input
                      value={jobTags}
                      onChange={e => setJobTags(e.target.value)}
                      placeholder="install,config"
                    />
                  </div>
                </div>
                <div className="mt-4 space-y-2">
                  <Label>Skip Tags</Label>
                  <Input
                    value={skipTags}
                    onChange={e => setSkipTags(e.target.value)}
                    placeholder="debug,tests"
                  />
                </div>
              </div>

              <div className="flex gap-2">
                <Button type="submit" disabled={createTemplate.isPending}>
                  {createTemplate.isPending ? 'Creating...' : 'Create'}
                </Button>
                <Button type="button" variant="outline" onClick={() => setShowCreate(false)}>
                  Cancel
                </Button>
              </div>
            </form>
          </CardContent>
        </Card>
      )}

      {isLoading ? (
        <LoadingState />
      ) : isError ? (
        <ErrorState message="Failed to load job templates" onRetry={() => refetch()} />
      ) : (
        <>
          <ListToolbar
            query={list.query}
            onQueryChange={list.setQuery}
            placeholder="Search job templates..."
          />
          {!list.pageItems.length ? (
            <Card>
              <CardContent>
                <EmptyState
                  message={list.query ? 'No job templates match your search.' : 'No job templates yet'}
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
                      <TableHead>Organization</TableHead>
                      <TableHead>Playbook</TableHead>
                      <TableHead>Inventory</TableHead>
                      <TableHead>Limit</TableHead>
                      <TableHead>Verbosity</TableHead>
                      <TableHead>Actions</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {list.pageItems.map(jt => (
                      <TableRow key={jt.id}>
                        <TableCell>
                          <div className="font-medium">{jt.name}</div>
                          {jt.description && (
                            <div className="text-xs text-muted-foreground">{jt.description}</div>
                          )}
                        </TableCell>
                        <TableCell>
                          {orgs?.results?.find(o => o.id === jt.organization_id)?.name ?? (
                            <Mono>{jt.organization_id.slice(0, 8)}</Mono>
                          )}
                        </TableCell>
                        <TableCell><Mono>{jt.playbook}</Mono></TableCell>
                        <TableCell>{jt.inventory_id ? 'Yes' : '—'}</TableCell>
                        <TableCell><Mono>{jt.limit || '—'}</Mono></TableCell>
                        <TableCell>{jt.verbosity > 0 ? `-${'v'.repeat(jt.verbosity)}` : 'Normal'}</TableCell>
                        <TableCell>
                          <Button
                            size="sm"
                            onClick={() => handleLaunch(jt.id)}
                            disabled={launchJob.isPending}
                          >
                            Launch
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
