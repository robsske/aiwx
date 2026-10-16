import { useState } from 'react'
import { useCredentials, useCredentialTypes, useOrganizations, useCreateCredential, useDeleteCredential } from '../hooks/useApi'
import { ErrorState } from '../components/ErrorState'
import { ConfirmDialog } from '../components/ConfirmDialog'
import { PageHeader, LoadingState, EmptyState } from '../components/Page'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { ListToolbar, ListPagination, useListControls } from '@/hooks/useListControls'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

function extractErrorMessage(err: unknown): string {
  if (err && typeof err === 'object' && 'response' in err) {
    const resp = (err as { response?: { data?: { message?: string } } }).response
    if (resp?.data?.message) return resp.data.message
  }
  if (err instanceof Error) return err.message
  return 'An unexpected error occurred'
}

export default function Credentials() {
  const { data: credentials, isLoading, isError, error, refetch } = useCredentials()
  const { data: credentialTypes, isLoading: typesLoading } = useCredentialTypes()
  const { data: orgs } = useOrganizations()
  const createCredential = useCreateCredential()
  const deleteCredential = useDeleteCredential()
  const [showCreate, setShowCreate] = useState(false)
  const [name, setName] = useState('')
  const [typeId, setTypeId] = useState('')
  const [inputs, setInputs] = useState('')
  const [formError, setFormError] = useState<string | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<string | null>(null)

  const selectedType = credentialTypes?.find((t) => t.id === typeId)

  const list = useListControls(credentials?.results, {
    getText: (c) => {
      const typeName = credentialTypes?.find((t) => t.id === c.credential_type_id)?.name ?? ''
      const org = orgs?.results?.find((o) => o.id === c.organization_id)?.name ?? ''
      return `${c.name} ${typeName} ${org}`
    },
    pageSize: 10,
  })

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    setFormError(null)

    if (!typeId) {
      setFormError('Please select a credential type')
      return
    }
    if (!name.trim()) {
      setFormError('Name is required')
      return
    }

    let parsedInputs: Record<string, unknown>
    try {
      parsedInputs = inputs.trim() ? JSON.parse(inputs) : {}
    } catch {
      setFormError('Invalid JSON for inputs')
      return
    }

    try {
      await createCredential.mutateAsync({
        name: name.trim(),
        credential_type_id: typeId,
        inputs: parsedInputs,
      })
      setShowCreate(false)
      setName('')
      setTypeId('')
      setInputs('')
    } catch (err: unknown) {
      setFormError(extractErrorMessage(err))
    }
  }

  const handleDelete = () => {
    if (deleteTarget) {
      deleteCredential.mutate(deleteTarget)
      setDeleteTarget(null)
    }
  }

  return (
    <div>
      <PageHeader
        title="Credentials"
        actions={<Button onClick={() => setShowCreate(true)}>+ New Credential</Button>}
      />

      {formError && <ErrorState message={formError} />}

      {showCreate && (
        <Card className="mb-6">
          <CardHeader>
            <CardTitle>Create Credential</CardTitle>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleCreate} className="flex flex-col gap-4">
              <div className="flex flex-col gap-2">
                <Label htmlFor="cred-name">Name</Label>
                <Input
                  id="cred-name"
                  type="text"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  required
                />
              </div>
              <div className="flex flex-col gap-2">
                <Label htmlFor="cred-type">Type</Label>
                {typesLoading ? (
                  <LoadingState label="Loading types..." />
                ) : (
                  <Select value={typeId || undefined} onValueChange={setTypeId}>
                    <SelectTrigger id="cred-type">
                      <SelectValue placeholder="Select type..." />
                    </SelectTrigger>
                    <SelectContent>
                      {credentialTypes?.map((t) => (
                        <SelectItem key={t.id} value={t.id}>
                          {t.name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                )}
              </div>
              <div className="flex flex-col gap-2">
                <Label htmlFor="cred-inputs">Inputs (JSON)</Label>
                <Textarea
                  id="cred-inputs"
                  className="font-mono text-xs"
                  value={inputs}
                  onChange={(e) => setInputs(e.target.value)}
                  placeholder={
                    selectedType
                      ? JSON.stringify(
                          Object.fromEntries(
                            Object.keys(selectedType.inputs_schema || {}).map((k) => [k, '']),
                          ),
                          null,
                          2,
                        )
                      : '{"username": "...", "password": "..."}'
                  }
                  rows={8}
                />
              </div>
              <div className="flex items-center gap-2">
                <Button
                  type="submit"
                  disabled={createCredential.isPending || !typeId || typesLoading}
                >
                  {createCredential.isPending ? 'Creating...' : 'Create'}
                </Button>
                <Button type="button" variant="secondary" onClick={() => setShowCreate(false)}>
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
        <ErrorState
          message={error instanceof Error ? error.message : 'Failed to load credentials'}
          onRetry={() => refetch()}
        />
      ) : (
        <>
          <ListToolbar
            query={list.query}
            onQueryChange={list.setQuery}
            placeholder="Search credentials..."
          />
          {!list.pageItems.length ? (
            <Card>
              <CardContent>
                <EmptyState
                  message={list.query ? 'No credentials match your search.' : 'No credentials yet.'}
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
                      <TableHead>Type</TableHead>
                      <TableHead>Created</TableHead>
                      <TableHead>Actions</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {list.pageItems.map((cred) => {
                      const typeName =
                        credentialTypes?.find((t) => t.id === cred.credential_type_id)?.name ??
                        cred.credential_type_id.slice(0, 8)
                      return (
                        <TableRow key={cred.id}>
                          <TableCell>{cred.name}</TableCell>
                          <TableCell>
                            {orgs?.results?.find(o => o.id === cred.organization_id)?.name ?? cred.organization_id.slice(0, 8)}
                          </TableCell>
                          <TableCell>{typeName}</TableCell>
                          <TableCell>{new Date(cred.created_at).toLocaleDateString()}</TableCell>
                          <TableCell>
                            <Button
                              variant="destructive"
                              size="sm"
                              onClick={() => setDeleteTarget(cred.id)}
                            >
                              Delete
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

      <ConfirmDialog
        open={!!deleteTarget}
        title="Delete Credential"
        message="Are you sure you want to delete this credential? This action cannot be undone."
        confirmLabel="Delete"
        danger
        onConfirm={handleDelete}
        onCancel={() => setDeleteTarget(null)}
      />
    </div>
  )
}
