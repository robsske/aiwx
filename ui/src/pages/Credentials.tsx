import { useEffect, useMemo, useState } from 'react'
import type { ReactNode } from 'react'
import YAML from 'yaml'
import {
  useCredentials,
  useCredentialTypes,
  useOrganizations,
  useCreateCredential,
  useDeleteCredential,
} from '../hooks/useApi'
import { api } from '../services/api'
import { ErrorState } from '../components/ErrorState'
import { ConfirmDialog } from '../components/ConfirmDialog'
import { PageHeader, LoadingState, EmptyState } from '../components/Page'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Checkbox } from '@/components/ui/checkbox'
import { Badge } from '@/components/ui/badge'
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

/** Parse YAML or JSON object text. Empty → {}. Throws on invalid. */
function parseStructured(text: string): Record<string, unknown> {
  const trimmed = text.trim()
  if (!trimmed) return {}
  if (trimmed.length > 64_000) {
    throw new Error('Input too large (max 64KB)')
  }
  const value = YAML.parse(trimmed, { maxAliasCount: 100 })
  if (value == null) return {}
  if (typeof value !== 'object' || Array.isArray(value)) {
    throw new Error('Expected a YAML/JSON mapping (key: value)')
  }
  return value as Record<string, unknown>
}

function toYAML(obj: unknown): string {
  return YAML.stringify(obj ?? {}, { indent: 2 })
}

const injectorOptions = [
  { key: 'extra_vars' as const, label: 'Extra vars', hint: 'Inject as --extra-vars' },
  { key: 'env' as const, label: 'Environment', hint: 'Inject as env vars' },
  { key: 'file' as const, label: 'File', hint: 'Write to a file path' },
]

const injectorKeys = ['extra_vars', 'env', 'file'] as const
type InjectorKey = (typeof injectorKeys)[number]

function selectedInjectorKeys(injectors: Record<string, unknown> | undefined): InjectorKey[] {
  if (!injectors || typeof injectors !== 'object') return []
  return injectorKeys.filter((k) => {
    const v = (injectors as Record<string, unknown>)[k]
    return v != null && !(typeof v === 'object' && !Array.isArray(v) && Object.keys(v as object).length === 0)
  })
}

function Monoish({ children }: { children: ReactNode }) {
  return <span className="font-mono text-xs text-muted-foreground">{children}</span>
}

export default function Credentials() {
  const { data: credentials, isLoading, isError, error, refetch } = useCredentials()
  const { data: credentialTypes, isLoading: typesLoading, refetch: refetchTypes } = useCredentialTypes()
  const { data: orgs } = useOrganizations()
  const createCredential = useCreateCredential()
  const deleteCredential = useDeleteCredential()

  const [tab, setTab] = useState<'credentials' | 'types'>('credentials')
  const [showCreate, setShowCreate] = useState(false)
  const [showCreateType, setShowCreateType] = useState(false)
  const [editingTypeId, setEditingTypeId] = useState<string | null>(null)

  const [name, setName] = useState('')
  const [typeId, setTypeId] = useState('')
  const [inputsYaml, setInputsYaml] = useState('')
  const [formError, setFormError] = useState<string | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<string | null>(null)

  const [typeName, setTypeName] = useState('')
  const [schemaYaml, setSchemaYaml] = useState('')
  const [injectorsYaml, setInjectorsYaml] = useState('')
  const [injectorKinds, setInjectorKinds] = useState<InjectorKey[]>(['extra_vars'])

  const selectedType = credentialTypes?.find((t) => t.id === typeId)

  // Seed default YAML keys when a type is selected (so submit sends real keys)
  useEffect(() => {
    if (!selectedType) return
    const keys = Object.keys(selectedType.inputs_schema || {})
    setInputsYaml(toYAML(Object.fromEntries(keys.map((k) => [k, '']))))
  }, [selectedType])

  const list = useListControls(credentials?.results, {
    getText: (c) => {
      const typeName = credentialTypes?.find((t) => t.id === c.credential_type_id)?.name ?? ''
      const org = orgs?.results?.find((o) => o.id === c.organization_id)?.name ?? ''
      return `${c.name} ${typeName} ${org}`
    },
    pageSize: 10,
  })

  const typeList = useListControls(credentialTypes, {
    getText: (t) => `${t.name} ${t.namespace}`,
    pageSize: 10,
  })

  const defaultInputsYaml = useMemo(() => {
    if (!selectedType) return '# YAML or JSON mapping\n'
    const keys = Object.keys(selectedType.inputs_schema || {})
    return toYAML(Object.fromEntries(keys.map((k) => [k, ''])))
  }, [selectedType])

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
      parsedInputs = parseStructured(inputsYaml)
    } catch (err) {
      setFormError(err instanceof Error ? `Invalid YAML/JSON for inputs: ${err.message}` : 'Invalid YAML/JSON for inputs')
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
      setInputsYaml('')
    } catch (err: unknown) {
      setFormError(extractErrorMessage(err))
    }
  }

  const resetTypeForm = () => {
    setTypeName('')
    setSchemaYaml('')
    setInjectorsYaml('')
    setInjectorKinds(['extra_vars'])
    setEditingTypeId(null)
    setShowCreateType(false)
  }

  const openEditType = (id: string) => {
    const t = credentialTypes?.find((x) => x.id === id)
    if (!t) return
    setEditingTypeId(id)
    setShowCreateType(true)
    setTypeName(t.name)
    setSchemaYaml(toYAML(t.inputs_schema ?? {}))
    const kinds = selectedInjectorKeys(t.injectors)
    setInjectorKinds(kinds)
    const inj = { ...(t.injectors ?? {}) }
    const body: Record<string, unknown> = {}
    for (const k of injectorKeys) {
      if (inj[k] != null) body[k] = inj[k]
    }
    setInjectorsYaml(toYAML(body))
  }

  const handleSaveType = async (e: React.FormEvent) => {
    e.preventDefault()
    setFormError(null)
    if (!typeName.trim()) {
      setFormError('Type name is required')
      return
    }

    let schema: Record<string, unknown>
    try {
      schema = parseStructured(schemaYaml)
    } catch (err) {
      setFormError(err instanceof Error ? `Invalid YAML/JSON schema: ${err.message}` : 'Invalid schema')
      return
    }

    let fullInjectors: Record<string, unknown>
    try {
      const parsed = parseStructured(injectorsYaml)
      fullInjectors = {}
      const hasMethodKeys = injectorKeys.some((k) => parsed[k] != null)
      for (const k of injectorKinds) {
        if (parsed[k] != null) {
          fullInjectors[k] = parsed[k]
        } else if (!hasMethodKeys && injectorKinds.length === 1 && injectorKinds[0] === 'extra_vars') {
          fullInjectors.extra_vars = parsed
        }
      }
    } catch (err) {
      setFormError(err instanceof Error ? `Invalid YAML/JSON injectors: ${err.message}` : 'Invalid injectors')
      return
    }

    try {
      if (editingTypeId) {
        await api.updateCredentialType(editingTypeId, {
          name: typeName.trim(),
          inputs_schema: schema,
          injectors: fullInjectors,
        })
      } else {
        await api.createCredentialType({
          name: typeName.trim(),
          inputs_schema: schema,
          injectors: fullInjectors,
        })
      }
      await refetchTypes()
      resetTypeForm()
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

  const toggleInjector = (key: InjectorKey, checked: boolean) => {
    setInjectorKinds((prev) => (checked ? [...prev, key] : prev.filter((k) => k !== key)))
  }

  return (
    <div>
      <PageHeader
        title="Credentials"
        actions={
          tab === 'credentials' ? (
            <Button onClick={() => setShowCreate(true)}>+ New Credential</Button>
          ) : (
            <Button
              onClick={() => {
                resetTypeForm()
                setShowCreateType(true)
              }}
            >
              + New Type
            </Button>
          )
        }
      />

      {formError && <ErrorState message={formError} />}

      <div className="mb-6 flex gap-2 border-b">
        <Button variant={tab === 'credentials' ? 'secondary' : 'ghost'} onClick={() => setTab('credentials')}>
          Credentials
        </Button>
        <Button variant={tab === 'types' ? 'secondary' : 'ghost'} onClick={() => setTab('types')}>
          Credential Types
        </Button>
      </div>

      {tab === 'credentials' && (
        <>
          {showCreate && (
            <Card className="mb-6">
              <CardHeader>
                <CardTitle>Create Credential</CardTitle>
              </CardHeader>
              <CardContent>
                <form onSubmit={handleCreate} className="flex flex-col gap-4">
                  <div className="flex flex-col gap-2">
                    <Label htmlFor="cred-name">Name</Label>
                    <Input id="cred-name" value={name} onChange={(e) => setName(e.target.value)} required />
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
                    <div className="flex items-center justify-between">
                      <Label htmlFor="cred-inputs">Inputs (YAML or JSON)</Label>
                      <Badge variant="outline">YAML</Badge>
                    </div>
                    <Textarea
                      id="cred-inputs"
                      className="font-mono text-xs"
                      value={inputsYaml}
                      onChange={(e) => setInputsYaml(e.target.value)}
                      placeholder={defaultInputsYaml}
                      rows={10}
                      spellCheck={false}
                    />
                    <p className="text-xs text-muted-foreground">
                      Accepts YAML or JSON. Secret fields are redacted in API responses.
                    </p>
                  </div>
                  <div className="flex items-center gap-2">
                    <Button type="submit" disabled={createCredential.isPending || !typeId || typesLoading}>
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
                          <TableHead>Injectors</TableHead>
                          <TableHead>Created</TableHead>
                          <TableHead>Actions</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {list.pageItems.map((cred) => {
                          const ctype = credentialTypes?.find((t) => t.id === cred.credential_type_id)
                          const inj = selectedInjectorKeys(ctype?.injectors)
                          return (
                            <TableRow key={cred.id}>
                              <TableCell>{cred.name}</TableCell>
                              <TableCell>
                                {orgs?.results?.find((o) => o.id === cred.organization_id)?.name ??
                                  cred.organization_id.slice(0, 8)}
                              </TableCell>
                              <TableCell>{ctype?.name ?? cred.credential_type_id.slice(0, 8)}</TableCell>
                              <TableCell>
                                <div className="flex flex-wrap gap-1">
                                  {inj.length === 0 && (
                                    <span className="text-xs text-muted-foreground">—</span>
                                  )}
                                  {inj.map((k) => (
                                    <Badge key={k} variant="outline" className="text-[10px]">
                                      {k}
                                    </Badge>
                                  ))}
                                </div>
                              </TableCell>
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
        </>
      )}

      {tab === 'types' && (
        <>
          {showCreateType && (
            <Card className="mb-6">
              <CardHeader>
                <CardTitle>
                  {editingTypeId ? 'Edit Credential Type' : 'Create Credential Type'}
                </CardTitle>
              </CardHeader>
              <CardContent>
                <form onSubmit={handleSaveType} className="flex flex-col gap-4">
                  <div className="flex flex-col gap-2">
                    <Label htmlFor="type-name">Name</Label>
                    <Input id="type-name" value={typeName} onChange={(e) => setTypeName(e.target.value)} required />
                  </div>

                  <div className="flex flex-col gap-2">
                    <div className="flex items-center justify-between">
                      <Label htmlFor="type-schema">Inputs schema (YAML or JSON)</Label>
                      <Badge variant="outline">YAML</Badge>
                    </div>
                    <Textarea
                      id="type-schema"
                      className="font-mono text-xs"
                      value={schemaYaml}
                      onChange={(e) => setSchemaYaml(e.target.value)}
                      placeholder={toYAML({
                        username: { type: 'string' },
                        password: { type: 'string', secret: true },
                      })}
                      rows={8}
                      spellCheck={false}
                    />
                  </div>

                  <div className="space-y-3 rounded-lg border p-4">
                    <div>
                      <Label className="text-sm font-medium">Injector methods</Label>
                      <p className="text-xs text-muted-foreground">
                        How credential fields are injected into job runs (AWX-style).
                      </p>
                    </div>

                    <div className="flex flex-wrap items-center gap-4">
                      <label className="flex items-center gap-2 text-sm">
                        <Checkbox
                          checked={injectorKinds.length === injectorKeys.length}
                          onCheckedChange={(c) => setInjectorKinds(c === true ? [...injectorKeys] : [])}
                        />
                        <span className="font-medium">All</span>
                      </label>
                      {injectorOptions.map((opt) => (
                        <label key={opt.key} className="flex items-center gap-2 text-sm">
                          <Checkbox
                            checked={injectorKinds.includes(opt.key)}
                            onCheckedChange={(c) => toggleInjector(opt.key, c === true)}
                          />
                          <span>
                            {opt.label}
                            <span className="ml-1 text-xs text-muted-foreground">({opt.hint})</span>
                          </span>
                        </label>
                      ))}
                    </div>

                    <div className="flex flex-col gap-2">
                      <Label htmlFor="type-injectors">Injector mapping (YAML or JSON)</Label>
                      <Textarea
                        id="type-injectors"
                        className="font-mono text-xs"
                        value={injectorsYaml}
                        onChange={(e) => setInjectorsYaml(e.target.value)}
                        placeholder={toYAML({
                          extra_vars: { api_token: '{{ vault_token }}' },
                          env: { CLOUD_KEY: '{{ secret_key }}' },
                          file: { '/tmp/id_rsa': '{{ ssh_key }}' },
                        })}
                        rows={10}
                        spellCheck={false}
                      />
                      <p className="text-xs text-muted-foreground">
                        Only methods checked above are saved. Use {'{{ field_name }}'} to reference input fields.
                      </p>
                    </div>
                  </div>

                  <div className="flex items-center gap-2">
                    <Button type="submit">{editingTypeId ? 'Save Type' : 'Create Type'}</Button>
                    <Button type="button" variant="secondary" onClick={resetTypeForm}>
                      Cancel
                    </Button>
                  </div>
                </form>
              </CardContent>
            </Card>
          )}

          {typesLoading ? (
            <LoadingState />
          ) : (
            <>
              <ListToolbar
                query={typeList.query}
                onQueryChange={typeList.setQuery}
                placeholder="Search credential types..."
              />
              <Card>
                <CardContent className="p-0">
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>Name</TableHead>
                        <TableHead>Namespace</TableHead>
                        <TableHead>Input fields</TableHead>
                        <TableHead>Injectors</TableHead>
                        <TableHead>Actions</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {typeList.pageItems.map((t) => {
                        const inj = selectedInjectorKeys(t.injectors)
                        return (
                          <TableRow key={t.id}>
                            <TableCell className="font-medium">{t.name}</TableCell>
                            <TableCell>
                              <Monoish>{t.namespace}</Monoish>
                            </TableCell>
                            <TableCell>
                              <div className="flex flex-wrap gap-1">
                                {Object.keys(t.inputs_schema ?? {}).map((k) => (
                                  <Badge key={k} variant="secondary" className="text-[10px]">
                                    {k}
                                  </Badge>
                                ))}
                              </div>
                            </TableCell>
                            <TableCell>
                              <div className="flex flex-wrap gap-1">
                                {inj.length === 0 && (
                                  <span className="text-xs text-muted-foreground">none</span>
                                )}
                                {inj.map((k) => (
                                  <Badge key={k} variant="outline" className="text-[10px]">
                                    {k}
                                  </Badge>
                                ))}
                              </div>
                            </TableCell>
                            <TableCell>
                              <Button size="sm" variant="outline" onClick={() => openEditType(t.id)}>
                                Edit
                              </Button>
                            </TableCell>
                          </TableRow>
                        )
                      })}
                    </TableBody>
                  </Table>
                </CardContent>
              </Card>
              <ListPagination
                page={typeList.page}
                totalPages={typeList.totalPages}
                pageSize={typeList.pageSize}
                total={typeList.total}
                rangeLabel={typeList.rangeLabel}
                onPageChange={typeList.setPage}
                onPageSizeChange={typeList.setPageSize}
              />
            </>
          )}
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
