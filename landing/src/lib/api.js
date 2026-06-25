const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || '/api'
const AUTH_BASE_URL = import.meta.env.VITE_AUTH_BASE_URL || '/auth'

async function request(url, options = {}) {
  const res = await fetch(url, {
    credentials: 'include',
    headers: { 'Content-Type': 'application/json', ...options.headers },
    ...options,
  })
  if (!res.ok) {
    const text = await res.text()
    throw new Error(text || `Request failed with status ${res.status}`)
  }
  if (res.status === 204) return null
  return res.json()
}

export function telegramLogin(payload) {
  return request(`${AUTH_BASE_URL}/telegram/callback`, {
    headers: new Headers({'content-type': 'application/json'}),
    method: 'POST',
    body: JSON.stringify(payload),
  })
}

export function logout() {
  return request(`${AUTH_BASE_URL}/logout`, { method: 'POST' })
}

export function listChats() {
  return request(`${API_BASE_URL}/chats`)
}

export function listSummaries(chatId) {
  return request(`${API_BASE_URL}/chats/${chatId}/summaries`)
}

export function listActionItems(chatId, status = '') {
  const qs = status ? `?status=${encodeURIComponent(status)}` : ''
  return request(`${API_BASE_URL}/chats/${chatId}/action-items${qs}`)
}

export function updateActionItemStatus(id, status) {
  return request(`${API_BASE_URL}/action-items/${id}`, {
    method: 'PATCH',
    body: JSON.stringify({ status }),
  })
}

export function searchMessages(chatId, query, mode = '') {
  const params = new URLSearchParams({ q: query, ...(mode ? { mode } : {}) })
  return request(`${API_BASE_URL}/chats/${chatId}/search?${params.toString()}`)
}

export function getNotionStatus(chatId) {
  return request(`${API_BASE_URL}/chats/${chatId}/notion`)
}

export function listNotionDatabases(chatId) {
  return request(`${API_BASE_URL}/chats/${chatId}/notion/databases`)
}

export function setNotionDatabase(chatId, databaseId) {
  return request(`${API_BASE_URL}/chats/${chatId}/notion/database`, {
    method: 'PATCH',
    body: JSON.stringify({ database_id: databaseId }),
  })
}

// notionOAuthStartHref returns an absolute URL for the "Connect Notion" link.
// This must be a real browser navigation (not a fetch), since the API
// responds with a redirect to Notion -- so a relative AUTH_BASE_URL (the
// default, when frontend and API share an origin via a reverse proxy) has to
// be resolved against the current page's origin, not assumed to already be
// absolute. Resolving it here, from the same AUTH_BASE_URL used to build the
// path, keeps this in sync with telegramLogin/logout instead of letting a
// caller independently (and possibly wrongly) compute an API origin.
export function notionOAuthStartHref(chatId) {
  return new URL(`${AUTH_BASE_URL}/notion/start/${chatId}`, window.location.origin).toString()
}
