import type { ReactNode } from 'react'
import { Badge } from './ui/badge'
import { cn } from '@/lib/utils'

export function PageHeader({
  title,
  actions,
}: {
  title: string
  actions?: ReactNode
}) {
  return (
    <div className="mb-6 flex flex-wrap items-center justify-between gap-3">
      <h1 className="text-2xl font-semibold tracking-tight">{title}</h1>
      {actions && <div className="flex flex-wrap items-center gap-2">{actions}</div>}
    </div>
  )
}

const statusVariant: Record<string, 'success' | 'warning' | 'destructive' | 'info' | 'muted' | 'secondary'> = {
  successful: 'success',
  success: 'success',
  active: 'success',
  running: 'info',
  pending: 'warning',
  waiting: 'warning',
  new: 'muted',
  canceled: 'muted',
  failed: 'destructive',
  error: 'destructive',
  never_updated: 'muted',
}

export function StatusBadge({ status, className }: { status: string; className?: string }) {
  return (
    <Badge variant={statusVariant[status] ?? 'secondary'} className={cn('capitalize', className)}>
      {status.replace(/_/g, ' ')}
    </Badge>
  )
}

export function EmptyState({ message }: { message: string }) {
  return (
    <div className="py-8 text-center text-sm text-muted-foreground">{message}</div>
  )
}

export function LoadingState({ label = 'Loading...' }: { label?: string }) {
  return <div className="py-8 text-center text-sm text-muted-foreground">{label}</div>
}

export function Mono({ children }: { children: ReactNode }) {
  return <span className="font-mono text-xs text-muted-foreground">{children}</span>
}
