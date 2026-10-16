import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import {
  useWorkflowTemplate,
  useUpdateWorkflowTemplate,
  useLaunchWorkflow,
  useJobTemplates,
} from '../hooks/useApi'
import { ErrorState } from '../components/ErrorState'
import { PageHeader, LoadingState } from '../components/Page'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Badge } from '@/components/ui/badge'
import type { WorkflowNodeIn, WorkflowLinkIn } from '../types'

const NODE_W = 180
const NODE_H = 72

type LocalNode = WorkflowNodeIn & { id: string }
type LocalLink = WorkflowLinkIn

const conditionColor: Record<string, string> = {
  success: '#16a34a',
  failure: '#dc2626',
  always: '#2563eb',
}

function uuid() {
  return crypto.randomUUID()
}

export default function WorkflowBuilder() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { data: workflow, isLoading, isError, refetch } = useWorkflowTemplate(id!)
  const { data: templates } = useJobTemplates()
  const updateWorkflow = useUpdateWorkflowTemplate()
  const launchWorkflow = useLaunchWorkflow()

  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [nodes, setNodes] = useState<LocalNode[]>([])
  const [links, setLinks] = useState<LocalLink[]>([])
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [connectFrom, setConnectFrom] = useState<string | null>(null)
  const [condition, setCondition] = useState<'success' | 'failure' | 'always'>('success')
  const [addTemplateId, setAddTemplateId] = useState('')
  const [saving, setSaving] = useState(false)
  const [message, setMessage] = useState<string | null>(null)
  const canvasRef = useRef<HTMLDivElement>(null)
  const dragRef = useRef<{ id: string; ox: number; oy: number } | null>(null)

  useEffect(() => {
    if (!workflow) return
    setName(workflow.name)
    setDescription(workflow.description ?? '')
    setNodes(
      (workflow.nodes ?? []).map((n) => ({
        id: n.id,
        job_template_id: n.job_template_id,
        name: n.name,
        position_x: n.position_x,
        position_y: n.position_y,
      })),
    )
    setLinks(
      (workflow.links ?? []).map((l) => ({
        source_node_id: l.source_node_id,
        target_node_id: l.target_node_id,
        condition: l.condition,
      })),
    )
  }, [workflow])

  const templateName = useCallback(
    (tid: string | null | undefined) => {
      if (!tid) return 'Job Template'
      return templates?.results?.find((t) => t.id === tid)?.name ?? 'Job Template'
    },
    [templates],
  )

  const nodeLabel = useCallback(
    (n: LocalNode) => n.name || templateName(n.job_template_id),
    [templateName],
  )

  const addNode = () => {
    const tid = addTemplateId
    const n: LocalNode = {
      id: uuid(),
      job_template_id: tid || null,
      name: tid ? templateName(tid) : 'New Node',
      position_x: 80 + (nodes.length % 4) * 220,
      position_y: 80 + Math.floor(nodes.length / 4) * 120,
    }
    setNodes((prev) => [...prev, n])
    setSelectedId(n.id)
    setAddTemplateId('')
    setMessage(null)
  }

  const removeSelected = () => {
    if (!selectedId) return
    setNodes((prev) => prev.filter((n) => n.id !== selectedId))
    setLinks((prev) =>
      prev.filter((l) => l.source_node_id !== selectedId && l.target_node_id !== selectedId),
    )
    setSelectedId(null)
    setConnectFrom(null)
  }

  const onNodeClick = (id: string) => {
    if (connectFrom && connectFrom !== id) {
      const exists = links.some(
        (l) =>
          l.source_node_id === connectFrom &&
          l.target_node_id === id &&
          l.condition === condition,
      )
      if (!exists) {
        setLinks((prev) => [
          ...prev,
          { source_node_id: connectFrom, target_node_id: id, condition },
        ])
      }
      setConnectFrom(null)
      setMessage(null)
      return
    }
    setSelectedId(id)
  }

  const onPointerDown = (e: React.PointerEvent, id: string) => {
    const node = nodes.find((n) => n.id === id)
    if (!node) return
    dragRef.current = {
      id,
      ox: e.clientX - node.position_x,
      oy: e.clientY - node.position_y,
    }
    ;(e.target as HTMLElement).setPointerCapture?.(e.pointerId)
  }

  const onPointerMove = (e: React.PointerEvent) => {
    const drag = dragRef.current
    if (!drag) return
    const x = Math.max(0, e.clientX - drag.ox)
    const y = Math.max(0, e.clientY - drag.oy)
    setNodes((prev) =>
      prev.map((n) => (n.id === drag.id ? { ...n, position_x: x, position_y: y } : n)),
    )
  }

  const onPointerUp = () => {
    dragRef.current = null
  }

  const edgePath = (from: LocalNode, to: LocalNode) => {
    const x1 = from.position_x + NODE_W
    const y1 = from.position_y + NODE_H / 2
    const x2 = to.position_x
    const y2 = to.position_y + NODE_H / 2
    const mx = (x1 + x2) / 2
    return `M ${x1} ${y1} C ${mx} ${y1}, ${mx} ${y2}, ${x2} ${y2}`
  }

  const save = async () => {
    if (!id) return
    setSaving(true)
    setMessage(null)
    try {
      await updateWorkflow.mutateAsync({
        id,
        data: {
          name: name.trim(),
          description: description.trim(),
          nodes: nodes.map((n) => ({
            id: n.id,
            job_template_id: n.job_template_id,
            name: n.name,
            position_x: Math.round(n.position_x),
            position_y: Math.round(n.position_y),
          })),
          links,
        },
      })
      setMessage('Saved')
    } catch {
      setMessage('Save failed')
    } finally {
      setSaving(false)
    }
  }

  const launch = async () => {
    if (!id) return
    try {
      await save()
      await launchWorkflow.mutateAsync(id)
      navigate('/workflows')
    } catch {
      setMessage('Launch failed — save the workflow first')
    }
  }

  const selected = useMemo(
    () => nodes.find((n) => n.id === selectedId) ?? null,
    [nodes, selectedId],
  )

  if (isLoading && !workflow) return <LoadingState />
  if (isError && !workflow) {
    return <ErrorState message="Failed to load workflow" onRetry={() => refetch()} />
  }

  return (
    <div>
      <PageHeader
        title={name || 'Workflow Builder'}
        actions={
          <>
            <Button variant="secondary" onClick={() => navigate('/workflows')}>
              Back
            </Button>
            <Button variant="outline" onClick={save} disabled={saving}>
              {saving ? 'Saving...' : 'Save'}
            </Button>
            <Button onClick={launch} disabled={launchWorkflow.isPending}>
              {launchWorkflow.isPending ? 'Launching...' : 'Launch'}
            </Button>
          </>
        }
      />

      {message && (
        <div className="mb-4 text-sm text-muted-foreground">{message}</div>
      )}

      <div className="mb-4 grid gap-4 md:grid-cols-2">
        <div className="space-y-2">
          <Label htmlFor="wf-edit-name">Name</Label>
          <Input id="wf-edit-name" value={name} onChange={(e) => setName(e.target.value)} />
        </div>
        <div className="space-y-2">
          <Label htmlFor="wf-edit-desc">Description</Label>
          <Input
            id="wf-edit-desc"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
          />
        </div>
      </div>

      <div className="grid gap-4 lg:grid-cols-[280px_1fr]">
        {/* Palette / inspector */}
        <div className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Add Node</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              <div className="space-y-2">
                <Label>Job Template</Label>
                <Select value={addTemplateId || '__none__'} onValueChange={(v: string) => setAddTemplateId(v === '__none__' ? '' : v)}>
                  <SelectTrigger>
                    <SelectValue placeholder="Select template..." />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="__none__">Generic node</SelectItem>
                    {templates?.results?.map((t) => (
                      <SelectItem key={t.id} value={t.id}>
                        {t.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <Button className="w-full" onClick={addNode}>
                + Add Node
              </Button>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="text-base">Connect Nodes</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              <div className="space-y-2">
                <Label>Link condition</Label>
                <Select value={condition} onValueChange={(v: string) => setCondition(v as 'success' | 'failure' | 'always')}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="success">On success</SelectItem>
                    <SelectItem value="failure">On failure</SelectItem>
                    <SelectItem value="always">Always</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <p className="text-xs text-muted-foreground">
                Click source node, then target node to create a link.
              </p>
              <Button
                variant={connectFrom ? 'default' : 'outline'}
                className="w-full"
                disabled={!selectedId}
                onClick={() => {
                  setConnectFrom(selectedId)
                  setMessage(`Select target for ${condition} link…`)
                }}
              >
                {connectFrom ? 'Select target…' : 'Start link from selected'}
              </Button>
              {connectFrom && (
                <Button variant="ghost" className="w-full" onClick={() => setConnectFrom(null)}>
                  Cancel link
                </Button>
              )}
            </CardContent>
          </Card>

          {selected && (
            <Card>
              <CardHeader>
                <CardTitle className="text-base">Node</CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                <div className="space-y-2">
                  <Label>Name</Label>
                  <Input
                    value={selected.name}
                    onChange={(e) =>
                      setNodes((prev) =>
                        prev.map((n) =>
                          n.id === selected.id ? { ...n, name: e.target.value } : n,
                        ),
                      )
                    }
                  />
                </div>
                <div className="space-y-2">
                  <Label>Job Template</Label>
                  <Select
                    value={selected.job_template_id || '__none__'}
                    onValueChange={(v: string) =>
                      setNodes((prev) =>
                        prev.map((n) =>
                          n.id === selected.id
                            ? {
                                ...n,
                                job_template_id: v === '__none__' ? null : v,
                                name: v === '__none__' ? n.name : templateName(v),
                              }
                            : n,
                        ),
                      )
                    }
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="__none__">None</SelectItem>
                      {templates?.results?.map((t) => (
                        <SelectItem key={t.id} value={t.id}>
                          {t.name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <Button variant="destructive" className="w-full" onClick={removeSelected}>
                  Delete Node
                </Button>
              </CardContent>
            </Card>
          )}

          <Card>
            <CardHeader>
              <CardTitle className="text-base">Links</CardTitle>
            </CardHeader>
            <CardContent className="space-y-2">
              {links.length === 0 && (
                <p className="text-xs text-muted-foreground">No links yet.</p>
              )}
              {links.map((l, i) => {
                const from = nodes.find((n) => n.id === l.source_node_id)
                const to = nodes.find((n) => n.id === l.target_node_id)
                return (
                  <div key={i} className="flex items-center justify-between gap-2 text-xs">
                    <span className="truncate">
                      {from ? nodeLabel(from) : '?'} → {to ? nodeLabel(to) : '?'}
                    </span>
                    <Badge
                      variant="outline"
                      style={{ color: conditionColor[l.condition], borderColor: conditionColor[l.condition] }}
                    >
                      {l.condition}
                    </Badge>
                    <Button
                      size="sm"
                      variant="ghost"
                      onClick={() => setLinks((prev) => prev.filter((_, j) => j !== i))}
                    >
                      ×
                    </Button>
                  </div>
                )
              })}
            </CardContent>
          </Card>
        </div>

        {/* Canvas */}
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Builder</CardTitle>
          </CardHeader>
          <CardContent className="p-0">
            <div
              ref={canvasRef}
              className="relative h-[560px] w-full overflow-auto bg-muted/30"
              onPointerMove={onPointerMove}
              onPointerUp={onPointerUp}
              style={{
                backgroundImage:
                  'radial-gradient(circle, #cbd5e1 1px, transparent 1px)',
                backgroundSize: '20px 20px',
              }}
            >
              <svg className="pointer-events-none absolute inset-0 h-full w-full" style={{ minWidth: 1200, minHeight: 560 }}>
                {links.map((l, i) => {
                  const from = nodes.find((n) => n.id === l.source_node_id)
                  const to = nodes.find((n) => n.id === l.target_node_id)
                  if (!from || !to) return null
                  return (
                    <g key={i}>
                      <path
                        d={edgePath(from, to)}
                        fill="none"
                        stroke={conditionColor[l.condition] ?? '#64748b'}
                        strokeWidth={2}
                        strokeDasharray={l.condition === 'always' ? '6 4' : undefined}
                        markerEnd="url(#arrow)"
                      />
                      <text
                        x={(from.position_x + NODE_W + to.position_x) / 2}
                        y={(from.position_y + to.position_y) / 2 + NODE_H / 2 - 8}
                        textAnchor="middle"
                        className="fill-slate-600 text-[11px]"
                      >
                        {l.condition}
                      </text>
                    </g>
                  )
                })}
                <defs>
                  <marker id="arrow" markerWidth="8" markerHeight="8" refX="6" refY="3" orient="auto">
                    <path d="M0,0 L6,3 L0,6 Z" fill="#64748b" />
                  </marker>
                </defs>
              </svg>

              {nodes.map((n) => (
                <div
                  key={n.id}
                  onPointerDown={(e) => onPointerDown(e, n.id)}
                  onClick={() => onNodeClick(n.id)}
                  className={`absolute cursor-move rounded-lg border bg-white px-3 py-2 shadow-sm transition-shadow ${
                    selectedId === n.id
                      ? 'border-primary ring-2 ring-primary/30'
                      : connectFrom === n.id
                        ? 'border-amber-500 ring-2 ring-amber-300'
                        : 'border-border hover:shadow'
                  }`}
                  style={{
                    left: n.position_x,
                    top: n.position_y,
                    width: NODE_W,
                    height: NODE_H,
                  }}
                >
                  <div className="truncate text-sm font-semibold">{nodeLabel(n)}</div>
                  <div className="mt-1 flex items-center gap-1">
                    <Badge variant="secondary" className="text-[10px]">
                      job
                    </Badge>
                    {n.job_template_id && (
                      <span className="truncate text-[10px] text-muted-foreground">
                        {templateName(n.job_template_id)}
                      </span>
                    )}
                  </div>
                </div>
              ))}

              {nodes.length === 0 && (
                <div className="absolute inset-0 flex items-center justify-center text-sm text-muted-foreground">
                  Add a node to start building the workflow
                </div>
              )}
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
