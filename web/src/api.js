const BASE = '/api/v1'

export function getToken() { return localStorage.getItem('token') }

export async function api(path, opts = {}) {
  const headers = { 'Content-Type': 'application/json', ...(opts.headers || {}) }
  const t = getToken()
  if (t) headers['Authorization'] = 'Bearer ' + t
  const res = await fetch(BASE + path, { ...opts, headers })
  if (res.status === 401) {
    localStorage.removeItem('token')
    if (window.location.pathname !== '/login') window.location.href = '/login'
    throw new Error('unauthorized')
  }
  let data = {}
  try { data = await res.json() } catch (e) { data = {} }
  if (!res.ok) throw new Error(data.error || res.statusText || 'request failed')
  return data
}

export const login = (email, password) =>
  api('/auth/login', { method: 'POST', body: JSON.stringify({ email, password }) })
export const loginMFA = (mfa_token, code) =>
  api('/auth/login/mfa', { method: 'POST', body: JSON.stringify({ mfa_token, code }) })
export const mfaSetup = () => api('/auth/mfa/setup', { method: 'POST' })
export const mfaEnable = (code) =>
  api('/auth/mfa/enable', { method: 'POST', body: JSON.stringify({ code }) })
export const mfaDisable = (code) =>
  api('/auth/mfa/disable', { method: 'POST', body: JSON.stringify({ code }) })
export const me = () => api('/auth/me')
export const stats = () => api('/stats')

export function logout() {
  localStorage.removeItem('token')
  window.location.href = '/login'
}
