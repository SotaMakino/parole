@react.component
let make = (~onSuccess: unit => unit) => {
  let (mode, setMode) = React.useState(() => #login)
  let (username, setUsername) = React.useState(() => "")
  let (password, setPassword) = React.useState(() => "")
  let (error, setError) = React.useState(() => "")
  let (busy, setBusy) = React.useState(() => false)
  let (showPassword, setShowPassword) = React.useState(() => false)

  let submit = async e => {
    ReactEvent.Form.preventDefault(e)
    setBusy(_ => true)
    setError(_ => "")
    let result = switch mode {
    | #login => await AuthApi.login(~username, ~password)
    | #signup => await AuthApi.signup(~username, ~password)
    }
    switch result {
    | Ok() => onSuccess()
    | Error(err) => setError(_ => err.ApiClient.message)
    }
    setBusy(_ => false)
  }

  <div className="auth">
    <h1> {React.string(mode == #login ? "Log in" : "Sign up")} </h1>
    {error == "" ? React.null : <p className="error"> {React.string(error)} </p>}
    <form onSubmit={e => submit(e)->ignore}>
      <input
        value=username
        onChange={e => {
          let value = ReactEvent.Form.target(e)["value"]
          setUsername(_ => value)
        }}
        placeholder="Username"
        autoComplete="username"
        required=true
      />
      <div className="password-field">
        <input
          type_={showPassword ? "text" : "password"}
          value=password
          onChange={e => {
            let value = ReactEvent.Form.target(e)["value"]
            setPassword(_ => value)
          }}
          placeholder="Password"
          autoComplete={mode == #login ? "current-password" : "new-password"}
          minLength=8
          required=true
        />
        <button
          type_="button" className="toggle-password" onClick={_ => setShowPassword(s => !s)}>
          {React.string(showPassword ? "Hide" : "Show")}
        </button>
      </div>
      <button type_="submit" className="primary" disabled=busy>
        {React.string(
          busy ? "Please wait…" : mode == #login ? "Log in" : "Create account",
        )}
      </button>
    </form>
    <p>
      {React.string(mode == #login ? "No account? " : "Already have an account? ")}
      <button
        type_="button"
        className="link"
        onClick={_ => {
          setMode(m => m == #login ? #signup : #login)
          setError(_ => "")
        }}>
        {React.string(mode == #login ? "Sign up" : "Log in")}
      </button>
    </p>
  </div>
}
