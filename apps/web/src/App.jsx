import { useEffect, useState } from 'react'
import './App.css'

const API = 'http://localhost:8080'

function App() {
  const [users, setUsers] = useState([])
  const [id, setId] = useState('')
  const [name, setName] = useState('')
  const [error, setError] = useState('')

  async function loadUsers() {
    try {
      const res = await fetch(`${API}/users`)
      if (!res.ok) throw new Error(`server returned ${res.status}`)
      setUsers(await res.json())
      setError('')
    } catch (err) {
      setError(`Failed to load users: ${err.message}`)
    }
  }

  useEffect(() => {
    loadUsers()
  }, [])

  async function createUser(e) {
    e.preventDefault()
    try {
      const res = await fetch(`${API}/users`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id, name }),
      })
      if (!res.ok) {
        const body = await res.json().catch(() => ({}))
        throw new Error(body.error ?? `server returned ${res.status}`)
      }
      setId('')
      setName('')
      await loadUsers()
    } catch (err) {
      setError(`Failed to create user: ${err.message}`)
    }
  }

  async function renameUser(user) {
    const newName = window.prompt(`New name for ${user.name}:`, user.name)
    if (!newName || newName === user.name) return
    try {
      const res = await fetch(`${API}/users/${user.id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: newName }),
      })
      if (!res.ok) throw new Error(`server returned ${res.status}`)
      await loadUsers()
    } catch (err) {
      setError(`Failed to update user: ${err.message}`)
    }
  }

  return (
    <main className="app">
      <h1>Users</h1>

      {error && <p className="error">{error}</p>}

      <form onSubmit={createUser}>
        <input
          value={id}
          onChange={(e) => setId(e.target.value)}
          placeholder="ID"
          required
        />
        <input
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="Name"
          required
        />
        <button type="submit">Add</button>
      </form>

      {users.length === 0 ? (
        <p>No users yet.</p>
      ) : (
        <ul>
          {users.map((u) => (
            <li key={u.id}>
              <span>
                <code>{u.id}</code> {u.name}
              </span>
              <button type="button" onClick={() => renameUser(u)}>
                Rename
              </button>
            </li>
          ))}
        </ul>
      )}
    </main>
  )
}

export default App
