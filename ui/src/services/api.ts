import axios, { AxiosInstance, InternalAxiosRequestConfig } from 'axios'
import type {
  Organization, Project, Job, JobEvent, Credential, CredentialType,
  Inventory, JobTemplate, PaginatedResponse, UserInfo,
} from '../types'

// ─── API Client ───────────────────────────────────────────────────────────────

class ApiClient {
  private client: AxiosInstance

  constructor() {
    this.client = axios.create({
      baseURL: '/api/v1',
      headers: {
        'Content-Type': 'application/json',
      },
    })

    // Add auth interceptor
    this.client.interceptors.request.use((config: InternalAxiosRequestConfig) => {
      const token = localStorage.getItem('aiwx_token')
      if (token) {
        config.headers.Authorization = `Bearer ${token}`
      }
      return config
    })

    // Add error interceptor
    this.client.interceptors.response.use(
      (response) => response,
      (error) => {
        const isLoginRequest = error.config?.url?.includes('/auth/token')
        if (error.response?.status === 401 && !isLoginRequest) {
          localStorage.removeItem('aiwx_token')
          // Use pushState instead of full reload to preserve SPA state
          window.history.pushState({}, '', '/login')
          window.dispatchEvent(new PopStateEvent('popstate'))
        }
        return Promise.reject(error)
      },
    )
  }

  // ─── Auth ──────────────────────────────────────────────────────────────────

  async login(username: string, password: string): Promise<{ access_token: string }> {
    const response = await this.client.post('/auth/token', {
      grant_type: 'password',
      username,
      password,
    })
    return response.data
  }

  async getMe(): Promise<UserInfo> {
    const response = await this.client.get('/auth/me')
    return response.data
  }

  // ─── Organizations ─────────────────────────────────────────────────────────

  async listOrganizations(cursor?: string, limit = 50): Promise<PaginatedResponse<Organization>> {
    const response = await this.client.get('/organizations', {
      params: { cursor, limit },
    })
    return response.data
  }

  async getOrganization(id: string): Promise<Organization> {
    const response = await this.client.get(`/organizations/${id}`)
    return response.data
  }

  // ─── Projects ──────────────────────────────────────────────────────────────

  async listProjects(cursor?: string, limit = 50): Promise<PaginatedResponse<Project>> {
    const response = await this.client.get('/projects', {
      params: { cursor, limit },
    })
    return response.data
  }

  async getProject(id: string): Promise<Project> {
    const response = await this.client.get(`/projects/${id}`)
    return response.data
  }

  async createProject(data: Partial<Project>): Promise<Project> {
    const response = await this.client.post('/projects', data)
    return response.data
  }

  async updateProject(id: string, data: Partial<Project>): Promise<Project> {
    const response = await this.client.patch(`/projects/${id}`, data)
    return response.data
  }

  async deleteProject(id: string): Promise<void> {
    await this.client.delete(`/projects/${id}`)
  }

  async syncProject(id: string): Promise<void> {
    await this.client.post(`/projects/${id}/sync`)
  }

  // ─── Jobs ──────────────────────────────────────────────────────────────────

  async listJobs(cursor?: string, limit = 50): Promise<PaginatedResponse<Job>> {
    const response = await this.client.get('/jobs', {
      params: { cursor, limit },
    })
    return response.data
  }

  async getJob(id: string): Promise<Job> {
    const response = await this.client.get(`/jobs/${id}`)
    return response.data
  }

  async getJobEvents(id: string): Promise<PaginatedResponse<JobEvent>> {
    const response = await this.client.get(`/jobs/${id}/events`)
    return response.data
  }

  async getJobLogs(id: string): Promise<PaginatedResponse<{ sequence: number; content: string; source: string; created_at: string }>> {
    const response = await this.client.get(`/jobs/${id}/logs`)
    return response.data
  }

  async cancelJob(id: string): Promise<void> {
    await this.client.post(`/jobs/${id}/cancel`)
  }

  // ─── Credentials ───────────────────────────────────────────────────────────

  async listCredentials(cursor?: string, limit = 50): Promise<PaginatedResponse<Credential>> {
    const response = await this.client.get('/credentials', {
      params: { cursor, limit },
    })
    return response.data
  }

  async getCredential(id: string): Promise<Credential> {
    const response = await this.client.get(`/credentials/${id}`)
    return response.data
  }

  async createCredential(data: Partial<Credential>): Promise<Credential> {
    const response = await this.client.post('/credentials', data)
    return response.data
  }

  async deleteCredential(id: string): Promise<void> {
    await this.client.delete(`/credentials/${id}`)
  }

  async listCredentialTypes(): Promise<CredentialType[]> {
    const response = await this.client.get('/credential-types')
    return response.data.results ?? []
  }

  // ─── Inventories ───────────────────────────────────────────────────────────

  async listInventories(cursor?: string, limit = 50): Promise<PaginatedResponse<Inventory>> {
    const response = await this.client.get('/inventories', {
      params: { cursor, limit },
    })
    return response.data
  }

  async getInventory(id: string): Promise<Inventory> {
    const response = await this.client.get(`/inventories/${id}`)
    return response.data
  }

  async createInventory(data: Partial<Inventory>): Promise<Inventory> {
    const response = await this.client.post('/inventories', data)
    return response.data
  }

  async listInventoryHosts(id: string): Promise<PaginatedResponse<import('../types').InventoryHost>> {
    const response = await this.client.get(`/inventories/${id}/hosts`)
    return response.data
  }

  async createInventoryHost(id: string, data: { name: string; description?: string; variables?: Record<string, any> }): Promise<import('../types').InventoryHost> {
    const response = await this.client.post(`/inventories/${id}/hosts`, data)
    return response.data
  }

  async listInventoryGroups(id: string): Promise<PaginatedResponse<import('../types').InventoryGroup>> {
    const response = await this.client.get(`/inventories/${id}/groups`)
    return response.data
  }

  async createInventoryGroup(id: string, data: { name: string; description?: string; variables?: Record<string, any>; parent_group_id?: string }): Promise<import('../types').InventoryGroup> {
    const response = await this.client.post(`/inventories/${id}/groups`, data)
    return response.data
  }

  async deleteInventoryGroup(id: string, groupId: string): Promise<void> {
    await this.client.delete(`/inventories/${id}/groups/${groupId}`)
  }

  async addHostToGroup(id: string, groupId: string, hostId: string): Promise<void> {
    await this.client.post(`/inventories/${id}/groups/${groupId}/hosts`, { host_id: hostId })
  }

  async removeHostFromGroup(id: string, groupId: string, hostId: string): Promise<void> {
    await this.client.delete(`/inventories/${id}/groups/${groupId}/hosts/${hostId}`)
  }

  // ─── Job Templates ─────────────────────────────────────────────────────────

  async listJobTemplates(cursor?: string, limit = 50): Promise<PaginatedResponse<JobTemplate>> {
    const response = await this.client.get('/job-templates', {
      params: { cursor, limit },
    })
    return response.data
  }

  async getJobTemplate(id: string): Promise<JobTemplate> {
    const response = await this.client.get(`/job-templates/${id}`)
    return response.data
  }

  async createJobTemplate(data: Partial<JobTemplate>): Promise<JobTemplate> {
    const response = await this.client.post('/job-templates', data)
    return response.data
  }

  async launchJob(templateId: string, data?: Record<string, any>): Promise<Job> {
    const response = await this.client.post(`/job-templates/${templateId}/launch`, data)
    return response.data
  }
}

export const api = new ApiClient()
export default api
