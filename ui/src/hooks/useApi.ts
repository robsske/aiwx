import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../services/api'

// ─── Projects ─────────────────────────────────────────────────────────────────

export function useProjects() {
  return useQuery({
    queryKey: ['projects'],
    queryFn: () => api.listProjects(),
  })
}

export function useProject(id: string) {
  return useQuery({
    queryKey: ['projects', id],
    queryFn: () => api.getProject(id),
    enabled: !!id,
  })
}

export function useCreateProject() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: api.createProject,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects'] })
    },
  })
}

export function useSyncProject() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: api.syncProject,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects'] })
    },
  })
}

// ─── Jobs ─────────────────────────────────────────────────────────────────────

export function useJobs() {
  return useQuery({
    queryKey: ['jobs'],
    queryFn: () => api.listJobs(),
  })
}

export function useJob(id: string) {
  return useQuery({
    queryKey: ['jobs', id],
    queryFn: () => api.getJob(id),
    enabled: !!id,
    refetchInterval: 2000, // Poll every 2 seconds for live updates
  })
}

export function useJobEvents(id: string) {
  return useQuery({
    queryKey: ['jobs', id, 'events'],
    queryFn: () => api.getJobEvents(id),
    enabled: !!id,
    refetchInterval: 3000,
  })
}

export function useCancelJob() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: api.cancelJob,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['jobs'] })
    },
  })
}

// ─── Credentials ──────────────────────────────────────────────────────────────

export function useCredentialTypes() {
  return useQuery({
    queryKey: ['credential-types'],
    queryFn: () => api.listCredentialTypes(),
    staleTime: 5 * 60 * 1000,
  })
}

export function useCredentials() {
  return useQuery({
    queryKey: ['credentials'],
    queryFn: () => api.listCredentials(),
  })
}

export function useCreateCredential() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: api.createCredential,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['credentials'] })
    },
  })
}

export function useDeleteCredential() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: api.deleteCredential,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['credentials'] })
    },
  })
}

// ─── Inventories ──────────────────────────────────────────────────────────────

export function useInventories() {
  return useQuery({
    queryKey: ['inventories'],
    queryFn: () => api.listInventories(),
  })
}

export function useCreateInventory() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: api.createInventory,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['inventories'] })
    },
  })
}

// ─── Job Templates ────────────────────────────────────────────────────────────

export function useJobTemplates() {
  return useQuery({
    queryKey: ['job-templates'],
    queryFn: () => api.listJobTemplates(),
  })
}

export function useLaunchJob() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ templateId, data }: { templateId: string; data?: Record<string, any> }) =>
      api.launchJob(templateId, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['jobs'] })
    },
  })
}
