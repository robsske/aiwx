import { useParams } from 'react-router-dom'
import { useProject } from '../hooks/useApi'
import { PageHeader, StatusBadge, LoadingState, Mono } from '../components/Page'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { ErrorState } from '../components/ErrorState'

function DetailItem({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="space-y-1">
      <div className="text-xs font-medium text-muted-foreground">{label}</div>
      <div className="text-sm">{children}</div>
    </div>
  )
}

export default function ProjectDetail() {
  const { id } = useParams<{ id: string }>()
  const { data: project, isLoading } = useProject(id!)

  if (isLoading) {
    return <LoadingState />
  }

  if (!project) {
    return <ErrorState message="Project not found" />
  }

  return (
    <div>
      <PageHeader title={project.name} />

      <Card>
        <CardHeader>
          <CardTitle>Project Details</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            <DetailItem label="Git URL">
              <Mono>{project.scm_url}</Mono>
            </DetailItem>
            <DetailItem label="Branch">
              {project.scm_branch}
            </DetailItem>
            <DetailItem label="Sync Status">
              <StatusBadge status={project.sync_status} />
            </DetailItem>
            <DetailItem label="Latest Revision">
              <Mono>{project.latest_revision?.slice(0, 8) ?? '—'}</Mono>
            </DetailItem>
            <DetailItem label="Last Sync">
              {project.last_sync_at ? new Date(project.last_sync_at).toLocaleString() : '—'}
            </DetailItem>
            <DetailItem label="Created">
              {new Date(project.created_at).toLocaleString()}
            </DetailItem>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
