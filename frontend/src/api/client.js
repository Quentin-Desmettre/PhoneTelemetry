// Thin fetch wrapper. Cookies are same-origin (the Go server serves both the
// SPA and the API), so the session cookie rides along automatically.
async function req(method, url, body) {
  const opts = { method, headers: {}, credentials: 'same-origin' }
  if (body !== undefined) {
    opts.headers['Content-Type'] = 'application/json'
    opts.body = JSON.stringify(body)
  }
  const res = await fetch(url, opts)
  const text = await res.text()
  const data = text ? JSON.parse(text) : null
  if (!res.ok) {
    const err = new Error((data && data.error) || res.statusText)
    err.status = res.status
    throw err
  }
  return data
}

export const api = {
  status: () => req('GET', '/api/status'),
  setup: (username, password) => req('POST', '/api/setup', { username, password }),
  login: (username, password) => req('POST', '/api/login', { username, password }),
  logout: () => req('POST', '/api/logout'),
  current: () => req('GET', '/api/metrics/current'),
  history: (range) => req('GET', `/api/metrics/history?range=${encodeURIComponent(range)}`),
  getSettings: () => req('GET', '/api/settings'),
  putSettings: (s) => req('PUT', '/api/settings', s),
}
