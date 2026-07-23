%%raw(`import "./App.css"`)

// prompt = the word shown in full; tiles = the answer being spelled ("" = hidden).
// Which language is which depends on the round's direction.
type pair = {prompt: string, tiles: array<string>}

// guest = true means anonymous play; a signed-in account shows its name + count.
// plays is the global tally of rounds dealt (all players), shown as the issue N.
type me = {username: string, learned: int, guest: bool, plays: int}

type game = {
  id: int,
  status: string, // "playing" | "won" | "lost" ("lost" = flagged for review)
  direction: string, // "it" = spell the English word; "en" = spell the Italian one
  pairs: array<pair>,
  guessed: array<string>,
  results: array<bool>, // parallel to guessed: true = correct placement
  wrong: array<string>,
  usedUp: array<string>, // letters whose every occurrence is on the board
  maxMisses: int, // wrong placements allowed before the round is lost
}

@val @scope("window") external innerWidth: int = "innerWidth"
@val @scope("window") external innerHeight: int = "innerHeight"
@val @scope("window") external confirmDialog: string => bool = "confirm"

type keyboardEvent
@get external eventKey: keyboardEvent => string = "key"
@get external ctrlKey: keyboardEvent => bool = "ctrlKey"
@get external metaKey: keyboardEvent => bool = "metaKey"
@get external altKey: keyboardEvent => bool = "altKey"
@val @scope("document")
external addKeyListener: (string, keyboardEvent => unit) => unit = "addEventListener"
@val @scope("document")
external removeKeyListener: (string, keyboardEvent => unit) => unit = "removeEventListener"
@send external preventDefault: keyboardEvent => unit = "preventDefault"
type domTarget
@get external eventTarget: keyboardEvent => domTarget = "target"
@get external targetTag: domTarget => string = "tagName"
@get external targetEditable: domTarget => bool = "isContentEditable"

type pointerEvent
@val @scope("document")
external addPointerListener: (string, pointerEvent => unit) => unit = "addEventListener"
@val @scope("document")
external removePointerListener: (string, pointerEvent => unit) => unit = "removeEventListener"
type domNode
@get external pointerTarget: pointerEvent => domNode = "target"
@send @return(nullable) external closest: (domNode, string) => option<domNode> = "closest"
@send external blur: domNode => unit = "blur"
@val @scope("document") external activeElement: domNode = "activeElement"

let keyboardRows = [
  ["Q", "W", "E", "R", "T", "Y", "U", "I", "O", "P"],
  ["A", "S", "D", "F", "G", "H", "J", "K", "L"],
  ["Z", "X", "C", "V", "B", "N", "M"],
]

// revealed letters wear the Italian flag: il verde for repeated characters,
// il rosso for one-off ones (official tricolore values)
let uniqueColor = "#cd212a" // flag red
let repeatedColor = "#008c45" // flag green

@react.component
let make = () => {
  let (game, setGame) = React.useState(() => None)
  let (error, setError) = React.useState(() => "")
  let (notice, setNotice) = React.useState(() => "") // rejected letter, transient
  let (busy, setBusy) = React.useState(() => false)
  let (bursts, setBursts) = React.useState(() => [])
  let (shake, setShake) = React.useState(() => None) // tile flashed on a wrong drop
  let (account, setAccount) = React.useState(() => None) // fetched player: guest or account
  let (menuOpen, setMenuOpen) = React.useState(() => false)
  let (showAuth, setShowAuth) = React.useState(() => false) // sign-in overlay
  let (winDismissed, setWinDismissed) = React.useState(() => false) // win overlay closed
  let (winCount, setWinCount) = React.useState(() => 0) // learned tally, counting up
  let (uiLang, setUiLang) = React.useState(() => #it) // UI language, toggled by the flags
  let tr = I18n.strings(uiLang) // localized UI strings

  let celebrate = () => {
    let x = innerWidth / 2
    let y = innerHeight / 3
    let base = Js.Date.now()->Belt.Float.toInt
    // a small finale: main burst, then two smaller ones off to the sides
    let fire = (offsetX, offsetY, scale, afterMs, index) => {
      let key = base + index
      let _ = Js.Global.setTimeout(() => {
        setBursts(prev =>
          prev->Belt.Array.concat([Fireworks.makeBurst(x + offsetX, y + offsetY, scale, key)])
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

  let loadAccount = async () =>
    switch await ApiClient.request("/me") {
    | Ok(res) => {
        let fetched: me = await ApiClient.json(res)
        setAccount(_ => Some(fetched))
      }
    | Error(_) => ()
    }

  // the flags pick both the UI language and the guessing direction, so keep the
  // UI language in step with whatever direction the round came back with
  let applyGame = (g: game) => {
    setGame(_ => Some(g))
    setWinDismissed(_ => false) // a fresh round re-arms the win celebration
    setUiLang(_ => g.direction == "en" ? #en : #it)
  }

  let loadGame = async () => {
    setError(_ => "")
    switch await ApiClient.request("/game") {
    | Ok(res) => {
        let fetched: game = await ApiClient.json(res)
        applyGame(fetched)
        loadAccount()->ignore
      }
    | Error(err) => setError(_ => I18n.failedLoad(uiLang, err.message))
    }
  }

  React.useEffect0(() => {
    loadGame()->ignore
    None
  })

  // initial load failed: auto-retry every 5s instead of asking the user to click
  React.useEffect2(() => {
    switch game {
    | None if error != "" =>
      let id = Js.Global.setTimeout(() => loadGame()->ignore, 5000)
      Some(() => Js.Global.clearTimeout(id))
    | _ => None
    }
  }, (game, error))

  // the night-sky win overlay is up while the round is won and not dismissed
  let celebrating = switch game {
  | Some(g) => g.status == "won" && !winDismissed
  | None => false
  }
  let learned = switch account {
  | Some(acc) => acc.learned
  | None => 0
  }
  // only signed-in accounts may call the Cloud TTS endpoint; guests fall back to
  // the browser voice (see Speech.speakWord)
  let authenticated = switch account {
  | Some(acc) => !acc.guest
  | None => false
  }

  // tick the learned tally up from zero while the overlay is open
  React.useEffect2(() => {
    if celebrating && learned > 0 {
      setWinCount(_ => 0)
      let current = ref(0)
      let per = 1400 / learned
      let ms = per < 40 ? 40 : per
      let idRef = ref(None)
      let id = Js.Global.setInterval(() => {
        if current.contents >= learned {
          switch idRef.contents {
          | Some(i) => Js.Global.clearInterval(i)
          | None => ()
          }
        } else {
          current := current.contents + 1
          setWinCount(_ => current.contents)
        }
      }, ms)
      idRef := Some(id)
      Some(() => Js.Global.clearInterval(id))
    } else {
      setWinCount(_ => 0)
      None
    }
  }, (celebrating, learned))

  // keep the sky full of fireworks the whole time the overlay is open
  React.useEffect1(() => {
    if celebrating {
      let spawn = () => {
        let x = Js.Math.random_int(0, innerWidth)
        let y = Js.Math.random_int(innerHeight / 8, innerHeight * 3 / 5)
        let key = Js.Date.now()->Belt.Float.toInt + Js.Math.random_int(0, 100000)
        setBursts(prev =>
          prev->Belt.Array.concat([Fireworks.makeBurst(x, y, 0.7 +. Js.Math.random() *. 0.9, key)])
        )
        let _ = Js.Global.setTimeout(
          () => setBursts(prev => prev->Belt.Array.keep(b => b.key != key)),
          1500,
        )
      }
      let id = Js.Global.setInterval(() => {
        spawn()
        spawn()
      }, 250)
      Some(() => Js.Global.clearInterval(id))
    } else {
      None
    }
  }, [celebrating])

  let (selected, setSelected) = React.useState(() => "") // letter picked from the keyboard
  let (tileCursor, setTileCursor) = React.useState((): option<(int, int)> => None) // arrow-key cursor
  let (navMode, setNavMode) = React.useState(() => false) // true while navigating tiles by arrow keys

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
              let left = updated.maxMisses - updated.wrong->Belt.Array.length
              let shown = letter->Js.String2.toLowerCase
              setNotice(_ => I18n.notice(uiLang, shown, left))
              // shake the missed slot, then clear it so it can fire again
              setShake(_ => Some((wordIndex, position)))
              let _ = Js.Global.setTimeout(() => setShake(_ => None), 450)
            }
            if updated.status == "won" {
              celebrate()
              loadAccount()->ignore // refresh the learned tally for the win summary
            }
          }
        | Error(err) if err.status == 400 || err.status == 409 =>
          // the raw server hint ("tile already revealed") reads better in a
          // game notice than the full "HTTP 400: …" string
          setNotice(_ => err.message->Js.String2.replaceByRe(%re("/^HTTP \d+: /"), ""))
        | Error(err) => setError(_ => I18n.failedSubmit(uiLang, err.message))
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
        applyGame(fetched)
        loadAccount()->ignore // a new round bumps the global play tally (N.)
      }
    | Error(err) => setError(_ => I18n.failedStart(uiLang, err.message))
    }
    setBusy(_ => false)
  }

  let newGame = () => startRound("/game")

  // tapping a flag re-deals the untouched round in that direction (the server
  // rejects it once a letter is placed, but the UI disables the flags by then)
  let setDirection = async dir =>
    switch await ApiClient.request("/game/direction", ~method_="POST", ~body={"direction": dir}) {
    | Ok(res) => applyGame(await ApiClient.json(res))
    | Error(_) => ()
    }

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

  // --- keyboard-only placement: the arrow keys move a cursor across the open
  // tiles of the pairs, and Enter/Space drops the selected letter there. Only
  // the empty ("hidden input") slots take part; a pointer press hands control
  // back to the mouse (see the pointerdown effect below). ---

  // open positions (empty tiles) in a given pair row, as tile indices
  let openInRow = wi =>
    switch game {
    | Some(g) =>
      switch g.pairs->Belt.Array.get(wi) {
      | Some(p) =>
        p.tiles
        ->Belt.Array.mapWithIndex((pos, l) => (pos, l))
        ->Belt.Array.keep(((_, l)) => l == "")
        ->Belt.Array.map(((pos, _)) => pos)
      | None => []
      }
    | None => []
    }
  let rowCount = switch game {
  | Some(g) => g.pairs->Belt.Array.length
  | None => 0
  }
  let firstOpenTile = () => {
    let rec go = wi =>
      if wi >= rowCount {
        None
      } else {
        switch openInRow(wi)->Belt.Array.get(0) {
        | Some(pos) => Some((wi, pos))
        | None => go(wi + 1)
        }
      }
    go(0)
  }
  // the open tile the arrows point at, snapped to a valid slot as the board fills
  let activeTile = switch tileCursor {
  | Some((wi, pos)) if openInRow(wi)->Belt.Array.some(p => p == pos) => Some((wi, pos))
  | _ => firstOpenTile()
  }
  // nearest open tile in a row to a reference column, for vertical moves
  let nearestOpenInRow = (wi, refPos) =>
    switch openInRow(wi)->Belt.Array.get(0) {
    | None => None
    | Some(first) =>
      let best =
        openInRow(wi)->Belt.Array.reduce(first, (acc, p) =>
          Js.Math.abs_int(p - refPos) < Js.Math.abs_int(acc - refPos) ? p : acc
        )
      Some((wi, best))
    }
  let rec adjacentOpenRow = (wi, refPos, step) =>
    if wi < 0 || wi >= rowCount {
      None
    } else {
      switch nearestOpenInRow(wi, refPos) {
      | Some(t) => Some(t)
      | None => adjacentOpenRow(wi + step, refPos, step)
      }
    }
  // the closest open tile to one side (left/right) within the same row
  let closerOnSide = (opens, cpos, keepLeft) =>
    opens->Belt.Array.reduce(None, (acc, p) => {
      let onSide = keepLeft ? p < cpos : p > cpos
      if !onSide {
        acc
      } else {
        switch acc {
        | Some(a) =>
          let closer = keepLeft ? p > a : p < a
          closer ? Some(p) : acc
        | None => Some(p)
        }
      }
    })
  let moveTile = dir =>
    switch activeTile {
    | None => ()
    | Some((cwi, cpos)) =>
      let target = switch dir {
      | #left => closerOnSide(openInRow(cwi), cpos, true)->Belt.Option.map(p => (cwi, p))
      | #right => closerOnSide(openInRow(cwi), cpos, false)->Belt.Option.map(p => (cwi, p))
      | #up => adjacentOpenRow(cwi - 1, cpos, -1)
      | #down => adjacentOpenRow(cwi + 1, cpos, 1)
      }
      switch target {
      | Some(t) => setTileCursor(_ => Some(t))
      | None => ()
      }
    }
  let placeAtCursor = () =>
    switch activeTile {
    | Some((wi, pos)) => placeLetter(selected, wi, pos)->ignore
    | None => ()
    }
  let handleKeyEvent = e => {
    let target = e->eventTarget
    // never hijack the arrows while typing in a form field
    let editable =
      target->targetTag == "INPUT" || target->targetTag == "TEXTAREA" || target->targetEditable
    if editable {
      ()
    } else {
      switch e->eventKey {
      | "ArrowLeft" | "ArrowRight" | "ArrowUp" | "ArrowDown" =>
        e->preventDefault
        setNavMode(_ => true)
        let dir = switch e->eventKey {
        | "ArrowLeft" => #left
        | "ArrowRight" => #right
        | "ArrowUp" => #up
        | _ => #down
        }
        moveTile(dir)
      | "Enter" | " " =>
        if navMode {
          e->preventDefault
          placeAtCursor()
        }
      | "Escape" => setSelected(_ => "")
      | k => handleKey(k)
      }
    }
  }

  // a letter is "in hand" while dragging or while one is selected from the
  // keyboard; open slots light up so the player can see where it can go
  let (dragging, setDragging) = React.useState(() => false)
  let sensors = DndKit.useDefaultSensors()

  // dnd-kit reports the drop by ids: the dragged letter and the tile's
  // "wordIndex-position". A drag that ends off any tile leaves over null.
  let handleDragEnd = (e: DndKit.dragEndEvent) => {
    setDragging(_ => false)
    switch e.over->Js.Nullable.toOption {
    | Some(over) =>
      switch over.id->Js.String2.split("-") {
      | [ws, ps] =>
        switch (Belt.Int.fromString(ws), Belt.Int.fromString(ps)) {
        | (Some(wi), Some(pos)) => placeLetter(e.active.id, wi, pos)->ignore
        | _ => ()
        }
      | _ => ()
      }
    | None => ()
    }
  }

  // the physical keyboard listener mounts once, so route events through a ref
  // that always points at the latest render's handler
  let handleKeyRef = React.useRef(handleKeyEvent)
  handleKeyRef.current = handleKeyEvent

  React.useEffect0(() => {
    let listener = e =>
      if !(e->ctrlKey) && !(e->metaKey) && !(e->altKey) {
        handleKeyRef.current(e)
      }
    addKeyListener("keydown", listener)
    Some(() => removeKeyListener("keydown", listener))
  })

  // a pointer press (click or tap, anywhere) drops the arrow-key cursor so it
  // never lingers once the mouse takes over. A press outside the on-screen
  // keyboard also blurs any focused key, so its focus ring doesn't stick, and
  // drops the picked letter so the key and the armed slots lose their color.
  React.useEffect0(() => {
    let listener = e => {
      setNavMode(_ => false)
      switch e->pointerTarget->closest(".keyboard") {
      | Some(_) => () // inside the keyboard: leave the key focused
      | None =>
        switch activeElement->closest(".key") {
        | Some(key) => key->blur
        | None => ()
        }
        // an open tile keeps the letter (its click places it); anywhere else
        // clears the selection so nothing stays highlighted
        switch e->pointerTarget->closest(".tile.open") {
        | Some(_) => ()
        | None => setSelected(_ => "")
        }
      }
    }
    addPointerListener("pointerdown", listener)
    Some(() => removePointerListener("pointerdown", listener))
  })

  // signing in or out swaps the player identity, so reload the round (now keyed
  // on the account or the guest cookie) and refresh the account panel
  let afterAuthChange = () => {
    setShowAuth(_ => false)
    setMenuOpen(_ => false)
    loadGame()->ignore
  }

  let handleLogout = async () => {
    let _ = await AuthApi.logout()
    afterAuthChange()
  }

  // deleting the account wipes it server-side and drops the browser back to
  // anonymous guest play, so reuse the same reload path as signing out
  let handleDeleteAccount = async () =>
    if confirmDialog(tr.deleteConfirm) {
      let _ = await AuthApi.deleteAccount()
      afterAuthChange()
    }

  // open the account popup and refresh its learned-word count
  let toggleMenu = () => {
    let opening = !menuOpen
    setMenuOpen(_ => opening)
    if opening {
      loadAccount()->ignore
    }
  }

  switch game {
  | None =>
    // no game yet: still loading, or the initial load failed
    error == ""
      ? <main className="app">
          <div className="loading-screen">
            <div className="spinner" />
            <p> {React.string(tr.connecting)} </p>
          </div>
        </main>
      : <main className="app">
          <div className="loading-screen">
            <p className="error" role="alert"> {React.string(tr.serverWeak)} </p>
          </div>
        </main>
  | Some(g) =>
    <main className="app">
      <header className="app-header">
        <div className="dateline">
          <span>
            {React.string(
              switch account {
              | Some(acc) => NumberWords.issueLabel(uiLang, acc.plays)
              | None => uiLang == #it ? "N. —" : "No. —"
              },
            )}
          </span>
          <div className="dateline-meta">
            <span className="dateline-date"> {React.string(I18n.editionDate(uiLang))} </span>
            <span className="dateline-sep"> {React.string("|")} </span>
            {switch account {
            | Some(acc) if !acc.guest =>
              // signed in: name opens a popup with the vocabulary count + log out
              <div className="account">
                <button
                  type_="button"
                  className="username"
                  ariaLabel={tr.account}
                  onClick={_ => toggleMenu()}>
                  {React.string(acc.username)}
                </button>
                {!menuOpen
                  ? React.null
                  : <>
                      <div className="menu-backdrop" onClick={_ => setMenuOpen(_ => false)} />
                      <div className="account-menu" role="dialog">
                        <p className="menu-name"> {React.string(acc.username)} </p>
                        <div className="menu-stat">
                          <span className="menu-count">
                            {React.string(acc.learned->Belt.Int.toString)}
                          </span>
                          <span className="menu-label"> {React.string(tr.wordsLearned)} </span>
                        </div>
                        <div className="menu-actions">
                          <button
                            type_="button"
                            className="link menu-logout"
                            onClick={_ => handleLogout()->ignore}>
                            {React.string(tr.logOut)}
                          </button>
                          <button
                            type_="button"
                            className="link menu-delete"
                            onClick={_ => handleDeleteAccount()->ignore}>
                            {React.string(tr.deleteAccount)}
                          </button>
                        </div>
                      </div>
                    </>}
              </div>
            | _ =>
              // guest: a link to sign in and start tracking learned words
              <div className="account">
                <button type_="button" className="username" onClick={_ => setShowAuth(_ => true)}>
                  {React.string(tr.signIn)}
                </button>
              </div>
            }}
          </div>
        </div>
        <h1>
          {React.string("Le ")}
          <span className="cinque"> {React.string("Cinque")} </span>
        </h1>
        {
          // the flags choose the guessing direction, so they lock once the
          // round is under way — you can only switch on a fresh board
          let locked = g.guessed->Belt.Array.length > 0
          <div className="flag-row">
            <button
              type_="button"
              className={g.direction == "it" ? "flag active" : "flag"}
              ariaLabel={tr.showItalian}
              disabled=locked
              onClick={_ => setDirection("it")->ignore}>
              {React.string(`🇮🇹`)}
            </button>
            <span className="flag-sep"> {React.string("|")} </span>
            <button
              type_="button"
              className={g.direction == "en" ? "flag active" : "flag"}
              ariaLabel={tr.showEnglish}
              disabled=locked
              onClick={_ => setDirection("en")->ignore}>
              {React.string(`🇺🇸`)}
            </button>
          </div>
        }
      </header>
      {!showAuth
        ? React.null
        : <div className="auth-overlay">
            <div className="menu-backdrop" onClick={_ => setShowAuth(_ => false)} />
            <div className="auth-modal" role="dialog">
              <button
                type_="button"
                className="ghost auth-close"
                ariaLabel={tr.close}
                onClick={_ => setShowAuth(_ => false)}>
                {React.string("×")}
              </button>
              <AuthForm lang=uiLang onSuccess={() => afterAuthChange()} />
            </div>
          </div>}
      {error == "" ? React.null : <p className="error" role="alert"> {React.string(error)} </p>}
      {
        // a hit reveals a letter everywhere at once, so counting revealed
        // tiles gives each letter's true number of occurrences
        let letterCounts = {
          let m = Js.Dict.empty()
          g.pairs->Belt.Array.forEach(p =>
            p.tiles->Belt.Array.forEach(l =>
              if l != "" {
                m->Js.Dict.set(l, m->Js.Dict.get(l)->Belt.Option.getWithDefault(0) + 1)
              }
            )
          )
          m
        }
        // repeated characters show red, one-off characters green
        let tileColor = letter =>
          letterCounts->Js.Dict.get(letter)->Belt.Option.getWithDefault(0) > 1
            ? repeatedColor
            : uniqueColor
        let missCount = g.wrong->Belt.Array.length
        <DndKit.DndContext
          sensors
          onDragStart={_ => setDragging(_ => true)}
          onDragEnd={handleDragEnd}
          onDragCancel={() => setDragging(_ => false)}>
          <div className="pairs">
            {
              // the 🙊 pronounces the prompt word in its own language: Italian
              // when spelling English, English when spelling Italian
              let promptLang = g.direction == "en" ? "en-US" : "it-IT"
              g.pairs
              ->Belt.Array.mapWithIndex((wi, p) =>
                <div key=p.prompt className="pair-row">
                  <span className="italian">
                    <button
                      type_="button"
                      className="speak"
                      title={I18n.pronounce(uiLang, p.prompt)}
                      ariaLabel={I18n.pronounce(uiLang, p.prompt)}
                      onClick={_ => Speech.speakWord(p.prompt, promptLang, ~authenticated)}>
                      {React.string(`🙊`)}
                    </button>
                    {React.string(p.prompt)}
                  </span>
                  <div className="english-tiles">
                    {p.tiles
                    ->Belt.Array.mapWithIndex((i, letter) =>
                      letter == ""
                        ? {
                            let armed = selected != "" || dragging
                            <DndKit.Droppable
                              key={i->Belt.Int.toString}
                              dropId={`${wi->Belt.Int.toString}-${i->Belt.Int.toString}`}
                              className={"tile open" ++
                              (armed ? " armed" : "") ++
                              (shake == Some((wi, i)) ? " shake" : "") ++ (
                                navMode && activeTile == Some((wi, i)) ? " tile-cursor" : ""
                              )}
                              armed
                              onClick={_ => placeLetter(selected, wi, i)->ignore}
                            />
                          }
                        : <div
                            key={i->Belt.Int.toString}
                            className="tile revealed"
                            style={{backgroundColor: tileColor(letter)}}>
                            {React.string(letter)}
                          </div>
                    )
                    ->React.array}
                  </div>
                </div>
              )
              ->React.array
            }
          </div>
          // always rendered with reserved height, so guess feedback never
          // shifts the keyboard below it
          <p className="notice" role="alert"> {React.string(notice)} </p>
          <div className="tries">
            <span className="tries-label"> {React.string(tr.mistakes)} </span>
            {Belt.Array.makeBy(g.maxMisses, i =>
              <span
                key={i->Belt.Int.toString} className={i < missCount ? "try-dot spent" : "try-dot"}
              />
            )->React.array}
            <span className="tries-count">
              {React.string(`${missCount->Belt.Int.toString} / ${g.maxMisses->Belt.Int.toString}`)}
            </span>
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
                  <DndKit.Draggable
                    key=letter
                    letter
                    label=letter
                    className=cls
                    disabled=usedUp
                    dragDisabled={usedUp || g.status != "playing"}
                    onClick={_ => setSelected(s => s == letter ? "" : letter)}
                  />
                })
                ->React.array}
              </div>
            )
            ->React.array}
          </div>
          {
            // a loss keeps the inline banner (new game); a win gets the
            // full-screen night celebration overlay instead (rendered below)
            g.status == "lost"
              ? <div className="banner">
                  <p>
                    {React.string(
                      // stable per round (keyed on the game id), varied across rounds
                      switch tr.lostBanner->Belt.Array.get(
                        mod(g.id, Belt.Array.length(tr.lostBanner)),
                      ) {
                      | Some(m) => m
                      | None => ""
                      },
                    )}
                  </p>
                  <p className="saying">
                    {React.string(
                      switch tr.sayings->Belt.Array.get(mod(g.id, Belt.Array.length(tr.sayings))) {
                      | Some(s) => "“" ++ s ++ "”"
                      | None => ""
                      },
                    )}
                  </p>
                  <div className="banner-actions">
                    <button
                      type_="button"
                      className="primary"
                      disabled=busy
                      onClick={_ => newGame()->ignore}>
                      {React.string(tr.newGame)}
                    </button>
                  </div>
                </div>
              : React.null
          }
        </DndKit.DndContext>
      }
      <Fireworks bursts />
      {
        // night-sky celebration: dark backdrop, counting-up tally, and (via the
        // effects above) a sky full of fireworks that render on top of it
        !celebrating
          ? React.null
          : <div className="win-overlay">
              <button
                type_="button"
                className="win-close"
                ariaLabel={tr.close}
                onClick={_ => setWinDismissed(_ => true)}>
                {React.string("×")}
              </button>
              <div className="win-summary">
                <p className="win-message"> {React.string(tr.wonBanner)} </p>
                <div className="win-count">
                  <span className="win-count-num">
                    {React.string(winCount->Belt.Int.toString)}
                  </span>
                  <span className="win-count-label"> {React.string(tr.wordsLearned)} </span>
                </div>
                <button
                  type_="button"
                  className="primary win-new"
                  disabled=busy
                  onClick={_ => newGame()->ignore}>
                  {React.string(tr.newGame)}
                </button>
              </div>
            </div>
      }
      <footer className="app-footer">
        <p className="footer-links">
          <a href="/privacy.html"> {React.string(tr.privacy)} </a>
          <span className="footer-sep"> {React.string("|")} </span>
          <a href="/terms.html"> {React.string(tr.terms)} </a>
        </p>
      </footer>
    </main>
  }
}
