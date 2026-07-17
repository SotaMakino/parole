%%raw(`import "./App.css"`)

type todo = {id: int, title: string}

@react.component
let make = () => {
  let (authed, setAuthed) = React.useState(() => None) // None = still checking
  let (todos, setTodos) = React.useState(() => [])
  let (title, setTitle) = React.useState(() => "")
  let (error, setError) = React.useState(() => "")
  let (fieldError, setFieldError) = React.useState(() => "") // client-side validation
  let (busy, setBusy) = React.useState(() => false)

  let loadTodos = async () => {
    setError(_ => "")
    switch await ApiClient.request("/todos") {
    | Ok(res) => {
        let fetched: array<todo> = await ApiClient.json(res)
        setTodos(_ => fetched)
        setAuthed(_ => Some(true))
      }
    | Error(err) if err.status == 401 => setAuthed(_ => Some(false))
    | Error(err) => setError(_ => `Failed to load todos: ${err.message}`)
    }
  }

  React.useEffect0(() => {
    loadTodos()->ignore
    None
  })

  let addTodo = async e => {
    ReactEvent.Form.preventDefault(e)
    let trimmed = title->Js.String2.trim
    let isDuplicate =
      todos->Belt.Array.some(t =>
        t.title->Js.String2.toLowerCase == trimmed->Js.String2.toLowerCase
      )
    if trimmed == "" {
      setFieldError(_ => "A todo cannot be empty.")
    } else if isDuplicate {
      setFieldError(_ => `"${trimmed}" is already on your list.`)
    } else {
      setBusy(_ => true)
      switch await ApiClient.request("/todos", ~method_="POST", ~body={"title": trimmed}) {
      | Ok(_) => {
          setTitle(_ => "")
          await loadTodos()
        }
      | Error(err) => setError(_ => `Failed to add todo: ${err.message}`)
      }
      setBusy(_ => false)
    }
  }

  let deleteTodo = async (todo: todo) => {
    switch await ApiClient.request(`/todos/${todo.id->Belt.Int.toString}`, ~method_="DELETE") {
    | Ok(_) => await loadTodos()
    | Error(err) => setError(_ => `Failed to delete todo: ${err.message}`)
    }
  }

  let handleLogout = async () => {
    // even if the server is unreachable, drop back to the login screen
    let _ = await AuthApi.logout()
    setAuthed(_ => Some(false))
  }

  switch authed {
  | None =>
    // still checking the session; if the check itself failed, say so
    error == ""
      ? <main className="app">
          <div className="loading-screen">
            <div className="spinner" />
            <p> {React.string("Connecting to server…")} </p>
          </div>
        </main>
      : <main className="app">
          <p className="error" role="alert"> {React.string(error)} </p>
          <button type_="button" className="primary" onClick={_ => loadTodos()->ignore}>
            {React.string("Retry")}
          </button>
        </main>
  | Some(false) =>
    <main className="app">
      <AuthForm onSuccess={() => loadTodos()->ignore} />
    </main>
  | Some(true) =>
    <main className="app">
      <header className="app-header">
        <h1> {React.string("My todos")} </h1>
        <button type_="button" className="ghost" onClick={_ => handleLogout()->ignore}>
          {React.string("Log out")}
        </button>
      </header>
      {error == "" ? React.null : <p className="error" role="alert"> {React.string(error)} </p>}
      <form className="add-form" onSubmit={e => addTodo(e)->ignore}>
        <input
          value=title
          onChange={e => {
            let value = ReactEvent.Form.target(e)["value"]
            setTitle(_ => value)
            setFieldError(_ => "")
          }}
          placeholder="What needs doing?"
          ariaLabel="New todo"
        />
        <button type_="submit" className="primary" disabled=busy>
          {React.string("Add")}
        </button>
      </form>
      {fieldError == ""
        ? React.null
        : <p className="field-error" role="alert"> {React.string(fieldError)} </p>}
      {todos->Belt.Array.length == 0
        ? <p className="empty"> {React.string("Nothing to do. Add your first todo above.")} </p>
        : <ul className="todo-list">
            {todos
            ->Belt.Array.map(t =>
              <li key={t.id->Belt.Int.toString} className="todo-row">
                <span className="todo-title"> {React.string(t.title)} </span>
                <button
                  type_="button"
                  className="ghost small danger"
                  onClick={_ => deleteTodo(t)->ignore}>
                  {React.string("Delete")}
                </button>
              </li>
            )
            ->React.array}
          </ul>}
    </main>
  }
}
