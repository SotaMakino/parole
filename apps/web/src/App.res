%%raw(`import "./App.css"`)

type guess = {word: string, result: array<string>}

type game = {
  id: int,
  status: string, // "playing" | "won" | "lost"
  clue: string, // English meaning of the answer
  guesses: array<guess>,
  maxGuesses: int,
  wordLength: int,
  word: option<string>, // present only once the game is over
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
  ["ENTER", "Z", "X", "C", "V", "B", "N", "M", "BACK"],
]

// higher wins when a letter earned different feedback across guesses
let rank = s =>
  switch s {
  | "correct" => 3
  | "present" => 2
  | "absent" => 1
  | _ => 0
  }

@react.component
let make = () => {
  let (authed, setAuthed) = React.useState(() => None) // None = still checking
  let (game, setGame) = React.useState(() => None)
  let (current, setCurrent) = React.useState(() => "")
  let (error, setError) = React.useState(() => "")
  let (notice, setNotice) = React.useState(() => "") // rejected guess, transient
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

  let submitGuess = async () => {
    switch game {
    | Some(g) if g.status == "playing" && !busy =>
      if current->Js.String2.length < g.wordLength {
        setNotice(_ => "Not enough letters.")
      } else {
        setBusy(_ => true)
        switch await ApiClient.request("/game/guess", ~method_="POST", ~body={"guess": current}) {
        | Ok(res) => {
            let updated: game = await ApiClient.json(res)
            setGame(_ => Some(updated))
            setCurrent(_ => "")
            if updated.status == "won" {
              celebrate()
            }
          }
        | Error(err) if err.status == 401 => setAuthed(_ => Some(false))
        | Error(err) if err.status == 400 || err.status == 409 =>
          // the raw server hint ("not in the word list") reads better in a game
          // notice than the full "HTTP 400: …" string
          setNotice(_ => err.message->Js.String2.replaceByRe(%re("/^HTTP \d+: /"), ""))
        | Error(err) => setError(_ => `Failed to submit the guess: ${err.message}`)
        }
        setBusy(_ => false)
      }
    | _ => ()
    }
  }

  let newGame = async () => {
    setBusy(_ => true)
    setNotice(_ => "")
    switch await ApiClient.request("/game", ~method_="POST") {
    | Ok(res) => {
        let fetched: game = await ApiClient.json(res)
        setGame(_ => Some(fetched))
        setCurrent(_ => "")
      }
    | Error(err) if err.status == 401 => setAuthed(_ => Some(false))
    | Error(err) => setError(_ => `Failed to start a new game: ${err.message}`)
    }
    setBusy(_ => false)
  }

  let handleKey = k => {
    switch game {
    | Some(g) if g.status == "playing" && !busy =>
      if k == "Enter" {
        submitGuess()->ignore
      } else if k == "Backspace" {
        setNotice(_ => "")
        setCurrent(c => c->Js.String2.slice(~from=0, ~to_=c->Js.String2.length - 1))
      } else if k->Js.String2.length == 1 && %re("/^[a-z]$/i")->Js.Re.test_(k) {
        setNotice(_ => "")
        setCurrent(c => c->Js.String2.length < g.wordLength ? c ++ k->Js.String2.toUpperCase : c)
      }
    | _ => ()
    }
  }

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
            {React.string("Read the English clue — type the Italian word in 5 tries")}
          </p>
        </div>
        <button type_="button" className="ghost" onClick={_ => handleLogout()->ignore}>
          {React.string("Log out")}
        </button>
      </header>
      {error == "" ? React.null : <p className="error" role="alert"> {React.string(error)} </p>}
      {switch game {
      | None => React.null
      | Some(g) => {
          let submitted =
            g.guesses->Belt.Array.map(gu =>
              Belt.Array.makeBy(g.wordLength, i => (
                gu.word->Js.String2.charAt(i),
                "tile " ++ gu.result->Belt.Array.get(i)->Belt.Option.getWithDefault("absent"),
              ))
            )
          let currentRow =
            g.status == "playing"
              ? [
                  Belt.Array.makeBy(g.wordLength, i => {
                    let ch = current->Js.String2.charAt(i)
                    (ch, ch == "" ? "tile" : "tile filled")
                  }),
                ]
              : []
          let filled = Belt.Array.concat(submitted, currentRow)
          let empty = Belt.Array.makeBy(
            Js.Math.max_int(0, g.maxGuesses - filled->Belt.Array.length),
            _ => Belt.Array.makeBy(g.wordLength, _ => ("", "tile")),
          )
          let letterStatuses = {
            let m = Js.Dict.empty()
            g.guesses->Belt.Array.forEach(gu =>
              gu.result->Belt.Array.forEachWithIndex((i, r) => {
                let letter = gu.word->Js.String2.charAt(i)
                let prev = m->Js.Dict.get(letter)->Belt.Option.getWithDefault("")
                if rank(r) > rank(prev) {
                  m->Js.Dict.set(letter, r)
                }
              })
            )
            m
          }
          <>
            <div className="clue">
              <span className="clue-label"> {React.string("English")} </span>
              <span className="clue-text"> {React.string(g.clue)} </span>
            </div>
            <div className="board" ariaLabel="Game board">
              {Belt.Array.concat(filled, empty)
              ->Belt.Array.mapWithIndex((ri, row) =>
                <div key={ri->Belt.Int.toString} className="board-row">
                  {row
                  ->Belt.Array.mapWithIndex((ti, (letter, cls)) =>
                    <div key={ti->Belt.Int.toString} className=cls> {React.string(letter)} </div>
                  )
                  ->React.array}
                </div>
              )
              ->React.array}
            </div>
            {notice == ""
              ? React.null
              : <p className="notice" role="alert"> {React.string(notice)} </p>}
            {g.status == "playing"
              ? React.null
              : <div className="banner">
                  <p>
                    {React.string(
                      g.status == "won"
                        ? `Bravo! ${g.word->Belt.Option.getWithDefault("")} means “${g.clue}”.`
                        : `The word was ${g.word->Belt.Option.getWithDefault(
                              "",
                            )} — “${g.clue}”. It will come back for review.`,
                    )}
                  </p>
                  <button
                    type_="button"
                    className="primary"
                    disabled=busy
                    onClick={_ => newGame()->ignore}>
                    {React.string("New game")}
                  </button>
                </div>}
            <div className="keyboard">
              {keyboardRows
              ->Belt.Array.mapWithIndex((ri, row) =>
                <div key={ri->Belt.Int.toString} className="kb-row">
                  {row
                  ->Belt.Array.map(k => {
                    let (label, keyValue, cls) = switch k {
                    | "ENTER" => ("Enter", "Enter", "key wide")
                    | "BACK" => ("⌫", "Backspace", "key wide")
                    | letter => (
                        letter,
                        letter,
                        letterStatuses
                        ->Js.Dict.get(letter)
                        ->Belt.Option.mapWithDefault("key", s => "key " ++ s),
                      )
                    }
                    <button key=k type_="button" className=cls onClick={_ => handleKey(keyValue)}>
                      {React.string(label)}
                    </button>
                  })
                  ->React.array}
                </div>
              )
              ->React.array}
            </div>
          </>
        }
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
