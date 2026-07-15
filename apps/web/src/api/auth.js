const API = import.meta.env.VITE_API_URL ?? 'http://localhost:8080'

async function post(path, body) {
  const res = await fetch(`${API}${path}`, {
    method: 'POST',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    body: body ? JSON.stringify(body) : undefined,
  })
  if (!res.ok) {
    const data = await res.json().catch(() => ({}))
    throw new Error(data.error ?? `server returned ${res.status}`)
  }
}

export async function signup(username, password) {
  await post('/signup', { username, password })
  await post('/login', { username, password })
}

export async function login(username, password) {
  await post('/login', { username, password })
}

export async function logout() {
  await post('/logout')
}
