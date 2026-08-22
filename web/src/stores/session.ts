import { defineStore } from 'pinia'
import { request } from '../services/http'

interface Tokens { access_token: string; refresh_token: string; access_expires_at: string }

export const useSessionStore = defineStore('session', {
  state: () => ({
    accessToken: sessionStorage.getItem('access_token') ?? '',
    refreshToken: localStorage.getItem('refresh_token') ?? '',
    siteID: localStorage.getItem('site_id') ?? 'site-demo'
  }),
  getters: { authenticated: state => state.accessToken !== '' },
  actions: {
    async login(username: string, password: string) {
      const tokens = await request<Tokens>('/auth/login', { method: 'POST', body: JSON.stringify({ username, password }) })
      this.accessToken = tokens.access_token
      this.refreshToken = tokens.refresh_token
      sessionStorage.setItem('access_token', tokens.access_token)
      localStorage.setItem('refresh_token', tokens.refresh_token)
      localStorage.setItem('site_id', this.siteID)
    },
    logout() {
      this.accessToken = ''
      this.refreshToken = ''
      sessionStorage.removeItem('access_token')
      localStorage.removeItem('refresh_token')
    }
  }
})
