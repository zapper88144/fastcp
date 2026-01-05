import type { Site, PHPInstance, Stats, LoginResponse, APIKey } from '@/types'

const API_BASE = '/api/v1'

class APIClient {
  private token: string | null = null

  constructor() {
    this.token = localStorage.getItem('fastcp_token')
  }

  setToken(token: string | null) {
    this.token = token
    if (token) {
      localStorage.setItem('fastcp_token', token)
    } else {
      localStorage.removeItem('fastcp_token')
    }
  }

  getToken(): string | null {
    return this.token
  }

  private async request<T>(
    endpoint: string,
    options: RequestInit = {}
  ): Promise<T> {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      ...((options.headers as Record<string, string>) || {}),
    }

    if (this.token) {
      headers['Authorization'] = `Bearer ${this.token}`
    }

    const response = await fetch(`${API_BASE}${endpoint}`, {
      ...options,
      headers,
    })

    if (response.status === 401) {
      this.setToken(null)
      window.location.href = '/login'
      throw new Error('Unauthorized')
    }

    const data = await response.json()

    if (!response.ok) {
      throw new Error(data.error || 'Request failed')
    }

    return data
  }

  // Auth
  async login(username: string, password: string): Promise<LoginResponse> {
    const data = await this.request<LoginResponse>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    })
    this.setToken(data.token)
    return data
  }

  logout() {
    this.setToken(null)
  }

  async getCurrentUser() {
    return this.request<{ id: string; username: string; role: string }>('/me')
  }

  // Sites
  async getSites(): Promise<{ sites: Site[]; total: number }> {
    return this.request('/sites')
  }

  async getSite(id: string): Promise<Site> {
    return this.request(`/sites/${id}`)
  }

  async createSite(site: Partial<Site>): Promise<Site> {
    return this.request('/sites', {
      method: 'POST',
      body: JSON.stringify(site),
    })
  }

  async updateSite(id: string, site: Partial<Site>): Promise<Site> {
    return this.request(`/sites/${id}`, {
      method: 'PUT',
      body: JSON.stringify(site),
    })
  }

  async deleteSite(id: string): Promise<void> {
    await this.request(`/sites/${id}`, { method: 'DELETE' })
  }

  async suspendSite(id: string): Promise<void> {
    await this.request(`/sites/${id}/suspend`, { method: 'POST' })
  }

  async unsuspendSite(id: string): Promise<void> {
    await this.request(`/sites/${id}/unsuspend`, { method: 'POST' })
  }

  async restartSiteWorkers(id: string): Promise<void> {
    await this.request(`/sites/${id}/restart-workers`, { method: 'POST' })
  }

  // PHP Instances
  async getPHPInstances(): Promise<{ instances: PHPInstance[]; total: number }> {
    return this.request('/php')
  }

  async getPHPInstance(version: string): Promise<PHPInstance> {
    return this.request(`/php/${version}`)
  }

  async startPHPInstance(version: string): Promise<void> {
    await this.request(`/php/${version}/start`, { method: 'POST' })
  }

  async stopPHPInstance(version: string): Promise<void> {
    await this.request(`/php/${version}/stop`, { method: 'POST' })
  }

  async restartPHPInstance(version: string): Promise<void> {
    await this.request(`/php/${version}/restart`, { method: 'POST' })
  }

  async restartPHPWorkers(version: string): Promise<void> {
    await this.request(`/php/${version}/restart-workers`, { method: 'POST' })
  }

  // Stats
  async getStats(): Promise<Stats> {
    return this.request('/stats')
  }

  // API Keys
  async getAPIKeys(): Promise<{ api_keys: APIKey[]; total: number }> {
    return this.request('/api-keys')
  }

  async createAPIKey(name: string, permissions: string[]): Promise<APIKey> {
    return this.request('/api-keys', {
      method: 'POST',
      body: JSON.stringify({ name, permissions }),
    })
  }

  async deleteAPIKey(id: string): Promise<void> {
    await this.request(`/api-keys/${id}`, { method: 'DELETE' })
  }

  // Admin
  async reloadAll(): Promise<void> {
    await this.request('/reload', { method: 'POST' })
  }
}

export const api = new APIClient()

