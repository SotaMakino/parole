%%raw(`import "./App.css"`)

type pair = {italian: string, english: array<string>} // "" = still hidden

type game = {
  id: int,
  status: string, // "playing" | "won" | "lost" ("lost" = flagged for review)
  pairs: array<pair>,
  guessed: array<string>,
  results: array<bool>, // parallel to guessed: true = correct placement
  wrong: array<string>,
  usedUp: array<string>, // letters whose every occurrence is on the board
}

// celebration fireworks: staggered bursts of randomized particles
type particle = {
  dx: float,
  dy: float,
  size: float,
  rot: float,
  color: string,
  delay: int,
  duration: int,
  streak: bool, // confetti streak instead of a round spark
}

type burst = {x: int, y: int, key: int, particles: array<particle>}

let burstColors = ["#aa3bff", "#f59e0b", "#ef4444", "#22c55e", "#06b6d4", "#ec4899", "#facc15"]

let makeBurst = (x, y, scale, key) => {
  let count = 24
  let particles = Belt.Array.makeBy(count, i => {
    let angle =
      2.0 *. Js.Math._PI *. Belt.Int.toFloat(i) /. Belt.Int.toFloat(count) +.
        (Js.Math.random() -. 0.5) *. 0.5
    let distance = (55.0 +. Js.Math.random() *. 65.0) *. scale
    {
      dx: Js.Math.cos(angle) *. distance,
      dy: Js.Math.sin(angle) *. distance,
      size: 4.0 +. Js.Math.random() *. 5.0,
      rot: Js.Math.random() *. 360.0,
      color: burstColors->Belt.Array.getExn(mod(i, Belt.Array.length(burstColors))),
      delay: Js.Math.random_int(0, 90),
      duration: 700 + Js.Math.random_int(0, 450),
      streak: mod(i, 3) == 0,
    }
  })
  {x, y, key, particles}
}

@val @scope("window") external innerWidth: int = "innerWidth"
@val @scope("window") external innerHeight: int = "innerHeight"

type keyboardEvent
@get external eventKey: keyboardEvent => string = "key"
@get external ctrlKey: keyboardEvent => bool = "ctrlKey"
@get external metaKey: keyboardEvent => bool = "metaKey"
@get external altKey: keyboardEvent => bool = "altKey"
@val @scope("document")
external addKeyListener: (string, keyboardEvent => unit) => unit = "addEventListener"
@val @scope("document")
external removeKeyListener: (string, keyboardEvent => unit) => unit = "removeEventListener"

let keyboardRows = [
  ["Q", "W", "E", "R", "T", "Y", "U", "I", "O", "P"],
  ["A", "S", "D", "F", "G", "H", "J", "K", "L"],
  ["Z", "X", "C", "V", "B", "N", "M"],
]

// every revealed occurrence of a letter shares that letter's color
let letterColors = [
  "#aa3bff",
  "#f59e0b",
  "#ef4444",
  "#22c55e",
  "#06b6d4",
  "#ec4899",
  "#8b5cf6",
  "#14b8a6",
  "#f97316",
  "#3b82f6",
  "#84cc16",
  "#e11d48",
  "#0ea5e9",
]

let colorFor = letter => {
  let code = letter->Js.String2.charCodeAt(0)->Belt.Float.toInt
  letterColors->Belt.Array.getExn(mod(code - 65, Belt.Array.length(letterColors)))
}

// HTML5 drag&drop: Firefox refuses to drag without setData, and the letter
// itself travels in a ref so the drop handler can read it synchronously
type dataTransfer
@get external dataTransfer: ReactEvent.Mouse.t => dataTransfer = "dataTransfer"
@send external setData: (dataTransfer, string, string) => unit = "setData"

@react.component
let make = () => {
  let (authed, setAuthed) = React.useState(() => None) // None = still checking
  let (game, setGame) = React.useState(() => None)
  let (error, setError) = React.useState(() => "")
  let (notice, setNotice) = React.useState(() => "") // rejected letter, transient
  let (busy, setBusy) = React.useState(() => false)
  let (bursts, setBursts) = React.useState(() => [])

  let celebrate = () => {
    let x = innerWidth / 2
    let y = innerHeight / 3
    let base = Js.Date.now()->Belt.Float.toInt
    // a small finale: main burst, then two smaller ones off to the sides
    let fire = (offsetX, offsetY, scale, afterMs, index) => {
      let key = base + index
      let _ = Js.Global.setTimeout(() => {
        setBursts(prev =>
          prev->Belt.Array.concat([makeBurst(x + offsetX, y + offsetY, scale, key)])
        )
        let _ = Js.Global.setTimeout(
          () => setBursts(prev => prev->Belt.Array.keep(b => b.key != key)),
          1400,
        )
      }, afterMs)
    }
    fire(0, 0, 1.2, 0, 0)
    fire(-75, -50, 0.8, 170, 1)
    fire(70, -65, 0.9, 340, 2)
  }

  let loadGame = async () => {
    setError(_ => "")
    switch await ApiClient.request("/game") {
    | Ok(res) => {
        let fetched: game = await ApiClient.json(res)
        setGame(_ => Some(fetched))
        setAuthed(_ => Some(true))
      }
    | Error(err) if err.status == 401 => setAuthed(_ => Some(false))
    | Error(err) => setError(_ => `Failed to load the game: ${err.message}`)
    }
  }

  React.useEffect0(() => {
    loadGame()->ignore
    None
  })

  let (selected, setSelected) = React.useState(() => "") // letter picked from the keyboard

  // place one letter on one exact tile
  let placeLetter = async (letter, wordIndex, position) => {
    switch game {
    | Some(g) if g.status == "playing" && !busy && letter != "" => {
        setBusy(_ => true)
        setNotice(_ => "")
        switch await ApiClient.request(
          "/game/guess",
          ~method_="POST",
          ~body={"guess": letter, "word": wordIndex, "position": position},
        ) {
        | Ok(res) => {
            let updated: game = await ApiClient.json(res)
            setGame(_ => Some(updated))
            // deselect a letter once its last tile is on the board
            setSelected(s => updated.usedUp->Belt.Array.some(l => l == s) ? "" : s)
            if updated.wrong->Belt.Array.length > g.wrong->Belt.Array.length {
              setNotice(_ => `No ${letter} there — that costs a miss.`)
            }
            if updated.status == "won" {
              celebrate()
            }
          }
        | Error(err) if err.status == 401 => setAuthed(_ => Some(false))
        | Error(err) if err.status == 400 || err.status == 409 =>
          // the raw server hint ("tile already revealed") reads better in a
          // game notice than the full "HTTP 400: …" string
          setNotice(_ => err.message->Js.String2.replaceByRe(%re("/^HTTP \d+: /"), ""))
        | Error(err) => setError(_ => `Failed to submit the letter: ${err.message}`)
        }
        setBusy(_ => false)
      }
    | _ => ()
    }
  }

  let startRound = async path => {
    setBusy(_ => true)
    setNotice(_ => "")
    switch await ApiClient.request(path, ~method_="POST") {
    | Ok(res) => {
        let fetched: game = await ApiClient.json(res)
        setGame(_ => Some(fetched))
      }
    | Error(err) if err.status == 401 => setAuthed(_ => Some(false))
    | Error(err) => setError(_ => `Failed to start a new game: ${err.message}`)
    }
    setBusy(_ => false)
  }

  let newGame = () => startRound("/game")
  let retryGame = () => startRound("/game/retry")

  // a physical key press picks the letter up; clicking a tile drops it
  let handleKey = k => {
    if k->Js.String2.length == 1 && %re("/^[a-z]$/i")->Js.Re.test_(k) {
      let letter = k->Js.String2.toUpperCase
      switch game {
      | Some(g) if g.status == "playing" && !(g.usedUp->Belt.Array.some(l => l == letter)) =>
        setSelected(s => s == letter ? "" : letter)
      | _ => ()
      }
    }
  }

  let dragged = React.useRef("")

  // the physical keyboard listener mounts once, so route events through a ref
  // that always points at the latest render's handler
  let handleKeyRef = React.useRef(handleKey)
  handleKeyRef.current = handleKey

  React.useEffect1(() => {
    switch authed {
    | Some(true) => {
        let listener = e =>
          if !(e->ctrlKey) && !(e->metaKey) && !(e->altKey) {
            handleKeyRef.current(e->eventKey)
          }
        addKeyListener("keydown", listener)
        Some(() => removeKeyListener("keydown", listener))
      }
    | _ => None
    }
  }, [authed])

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
          <button type_="button" className="primary" onClick={_ => loadGame()->ignore}>
            {React.string("Retry")}
          </button>
        </main>
  | Some(false) =>
    <main className="app">
      <AuthForm onSuccess={() => loadGame()->ignore} />
    </main>
  | Some(true) =>
    <main className="app">
      <header className="app-header">
        <div>
          <h1> {React.string("Parole")} </h1>
          <p className="tagline">
            {React.string(
              "Pick a letter and place it on its exact spot — drag it, or tap the letter then the tile",
            )}
          </p>
        </div>
        <button type_="button" className="ghost" onClick={_ => handleLogout()->ignore}>
          {React.string("Log out")}
        </button>
      </header>
      {error == "" ? React.null : <p className="error" role="alert"> {React.string(error)} </p>}
      {switch game {
      | None => React.null
      | Some(g) =>
        <>
          <div className="pairs">
            {g.pairs
            ->Belt.Array.mapWithIndex((wi, p) =>
              <div key=p.italian className="pair-row">
                <span className="italian"> {React.string(p.italian)} </span>
                <div className="english-tiles">
                  {p.english
                  ->Belt.Array.mapWithIndex((i, letter) =>
                    letter == ""
                      ? <div
                          key={i->Belt.Int.toString}
                          className="tile open"
                          onDragOver={e => ReactEvent.Mouse.preventDefault(e)}
                          onDrop={e => {
                            ReactEvent.Mouse.preventDefault(e)
                            let l = dragged.current
                            dragged.current = ""
                            placeLetter(l, wi, i)->ignore
                          }}
                          onClick={_ => placeLetter(selected, wi, i)->ignore}
                        />
                      : <div
                          key={i->Belt.Int.toString}
                          className="tile revealed"
                          style={{backgroundColor: colorFor(letter)}}>
                          {React.string(letter)}
                        </div>
                  )
                  ->React.array}
                </div>
              </div>
            )
            ->React.array}
          </div>
          {notice == ""
            ? React.null
            : <p className="notice" role="alert"> {React.string(notice)} </p>}
          <div className="typed">
            <span className="typed-label"> {React.string("Typed")} </span>
            {g.guessed->Belt.Array.length == 0
              ? <span className="typed-empty"> {React.string("no letters yet")} </span>
              : g.guessed
                ->Belt.Array.mapWithIndex((i, l) =>
                  g.results->Belt.Array.get(i)->Belt.Option.getWithDefault(false)
                    ? <span
                        key={i->Belt.Int.toString}
                        className="chip hit"
                        style={{backgroundColor: colorFor(l)}}>
                        {React.string(l)}
                      </span>
                    : <span key={i->Belt.Int.toString} className="chip miss">
                        {React.string(l)}
                      </span>
                )
                ->React.array}
          </div>
          <div className="keyboard">
            {keyboardRows
            ->Belt.Array.mapWithIndex((ri, row) =>
              <div key={ri->Belt.Int.toString} className="kb-row">
                {row
                ->Belt.Array.map(letter => {
                  // a fully placed letter leaves the keyboard for the board
                  let usedUp = g.usedUp->Belt.Array.some(l => l == letter)
                  let cls = switch (usedUp, selected == letter) {
                  | (true, _) => "key used"
                  | (false, true) => "key selected"
                  | _ => "key"
                  }
                  <button
                    key=letter
                    type_="button"
                    className=cls
                    disabled=usedUp
                    draggable={!usedUp && g.status == "playing"}
                    onDragStart={e => {
                      e->dataTransfer->setData("text/plain", letter)
                      dragged.current = letter
                    }}
                    onClick={_ => setSelected(s => s == letter ? "" : letter)}>
                    {React.string(letter)}
                  </button>
                })
                ->React.array}
              </div>
            )
            ->React.array}
          </div>
          {g.status == "playing"
            ? React.null
            : <div className="banner">
                <p>
                  {React.string(
                    g.status == "won"
                      ? "Bravo! You revealed all five words."
                      : "All revealed — but with more than 5 misses, so these words will come back for review.",
                  )}
                </p>
                <button
                  type_="button" className="ghost" disabled=busy onClick={_ => retryGame()->ignore}>
                  {React.string("Retry")}
                </button>
                <button
                  type_="button" className="primary" disabled=busy onClick={_ => newGame()->ignore}>
                  {React.string("New game")}
                </button>
              </div>}
        </>
      }}
      {bursts
      ->Belt.Array.map(b =>
        <div
          key={b.key->Belt.Int.toString}
          className="firework"
          ariaHidden=true
          style={{
            left: `${b.x->Belt.Int.toString}px`,
            top: `${b.y->Belt.Int.toString}px`,
          }}>
          {b.particles
          ->Belt.Array.mapWithIndex((i, p) => {
            let height = p.streak ? p.size *. 2.8 : p.size
            let base: ReactDOM.Style.t = {
              backgroundColor: p.color,
              width: `${p.size->Js.Float.toString}px`,
              height: `${height->Js.Float.toString}px`,
              boxShadow: `0 0 6px ${p.color}`,
              animationDelay: `${p.delay->Belt.Int.toString}ms`,
              animationDuration: `${p.duration->Belt.Int.toString}ms`,
            }
            let style =
              base
              ->ReactDOM.Style.unsafeAddProp("--dx", `${p.dx->Js.Float.toString}px`)
              ->ReactDOM.Style.unsafeAddProp("--dy", `${p.dy->Js.Float.toString}px`)
              ->ReactDOM.Style.unsafeAddProp("--rot", `${p.rot->Js.Float.toString}deg`)
            <span key={i->Belt.Int.toString} className={p.streak ? "streak" : "dot"} style />
          })
          ->React.array}
        </div>
      )
      ->React.array}
    </main>
  }
}
