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

// revealed letters wear the Italian flag: il verde for repeated characters,
// il rosso for one-off ones (official tricolore values)
let uniqueColor = "#cd212a" // flag red
let repeatedColor = "#008c45" // flag green

// HTML5 drag&drop: Firefox refuses to drag without setData, and the letter
// itself travels in a ref so the drop handler can read it synchronously
type dataTransfer
@get external dataTransfer: ReactEvent.Mouse.t => dataTransfer = "dataTransfer"
@send external setData: (dataTransfer, string, string) => unit = "setData"

// browser text-to-speech for Italian pronunciation
type utterance
@new external makeUtterance: string => utterance = "SpeechSynthesisUtterance"
@set external setLang: (utterance, string) => unit = "lang"
@set external setRate: (utterance, float) => unit = "rate"
@set external setPitch: (utterance, float) => unit = "pitch"
@val @scope(("window", "speechSynthesis"))
external speak: utterance => unit = "speak"
@val @scope(("window", "speechSynthesis"))
external cancelSpeech: unit => unit = "cancel"

// pronounce a word in the given BCP-47 voice ("it-IT" or "en-US")
let speakWord = (word, langCode) => {
  cancelSpeech() // cut off any word still playing
  let u = makeUtterance(word->Js.String2.toLowerCase)
  u->setLang(langCode)
  u->setRate(0.8) // slowed down so each syllable is easy to catch
  u->setPitch(1.0) // pitch
  speak(u)
}

// spell the play count for the masthead issue line (e.g. 130 → "centotrenta" /
// "one hundred thirty"), covering 0–9999; larger counts fall back to digits only
let italianUnits = [
  "zero",
  "uno",
  "due",
  "tre",
  "quattro",
  "cinque",
  "sei",
  "sette",
  "otto",
  "nove",
  "dieci",
  "undici",
  "dodici",
  "tredici",
  "quattordici",
  "quindici",
  "sedici",
  "diciassette",
  "diciotto",
  "diciannove",
]
let italianTens = [
  "",
  "",
  "venti",
  "trenta",
  "quaranta",
  "cinquanta",
  "sessanta",
  "settanta",
  "ottanta",
  "novanta",
]

// drop the trailing vowel of a tens/hundreds word before joining, e.g.
// venti+uno → ventuno, cento+otto → centotto
let dropLast = s => s->Js.String2.slice(~from=0, ~to_=s->Js.String2.length - 1)

let rec spellItalian = n =>
  if n < 20 {
    italianUnits->Belt.Array.getExn(n)
  } else if n < 100 {
    let base = italianTens->Belt.Array.getExn(n / 10)
    switch mod(n, 10) {
    | 0 => base
    | (1 | 8) as u => dropLast(base) ++ italianUnits->Belt.Array.getExn(u) // ventuno, ventotto
    | 3 => base ++ "tré" // ventitré
    | u => base ++ italianUnits->Belt.Array.getExn(u)
    }
  } else if n < 1000 {
    let rest = mod(n, 100)
    let prefix = n / 100 == 1 ? "cento" : italianUnits->Belt.Array.getExn(n / 100) ++ "cento"
    if rest == 0 {
      prefix
    } else {
      let word = spellItalian(rest)
      // merge the double o: cento+otto → centotto, cento+ottanta → centottanta
      word->Js.String2.charAt(0) == "o" ? dropLast(prefix) ++ word : prefix ++ word
    }
  } else if n < 10000 {
    let rest = mod(n, 1000)
    let prefix = n / 1000 == 1 ? "mille" : italianUnits->Belt.Array.getExn(n / 1000) ++ "mila"
    rest == 0 ? prefix : prefix ++ spellItalian(rest)
  } else {
    ""
  }

let englishUnits = [
  "zero",
  "one",
  "two",
  "three",
  "four",
  "five",
  "six",
  "seven",
  "eight",
  "nine",
  "ten",
  "eleven",
  "twelve",
  "thirteen",
  "fourteen",
  "fifteen",
  "sixteen",
  "seventeen",
  "eighteen",
  "nineteen",
]
let englishTens = [
  "",
  "",
  "twenty",
  "thirty",
  "forty",
  "fifty",
  "sixty",
  "seventy",
  "eighty",
  "ninety",
]

let rec spellEnglish = n =>
  if n < 20 {
    englishUnits->Belt.Array.getExn(n)
  } else if n < 100 {
    let tens = englishTens->Belt.Array.getExn(n / 10)
    mod(n, 10) == 0 ? tens : `${tens}-${englishUnits->Belt.Array.getExn(mod(n, 10))}` // sixty-nine
  } else if n < 1000 {
    let hundreds = `${englishUnits->Belt.Array.getExn(n / 100)} hundred`
    let rest = mod(n, 100)
    rest == 0 ? hundreds : `${hundreds} ${spellEnglish(rest)}`
  } else if n < 10000 {
    let thousands = `${englishUnits->Belt.Array.getExn(n / 1000)} thousand`
    let rest = mod(n, 1000)
    rest == 0 ? thousands : `${thousands} ${spellEnglish(rest)}`
  } else {
    ""
  }

// "N. 130 · centotrenta" / "No. 130 · one hundred thirty" (digits only past range)
let issueLabel = (lang, plays) => {
  let digits = plays->Belt.Int.toString
  let (prefix, word) = lang == #it ? ("N.", spellItalian(plays)) : ("No.", spellEnglish(plays))
  word == "" ? `${prefix} ${digits}` : `${prefix} ${digits} · ${word}`
}

@react.component
let make = () => {
  let (game, setGame) = React.useState(() => None)
  let (error, setError) = React.useState(() => "")
  let (notice, setNotice) = React.useState(() => "") // rejected letter, transient
  let (busy, setBusy) = React.useState(() => false)
  let (bursts, setBursts) = React.useState(() => [])
  let (account, setAccount) = React.useState(() => None) // fetched player: guest or account
  let (menuOpen, setMenuOpen) = React.useState(() => false)
  let (showAuth, setShowAuth) = React.useState(() => false) // sign-in overlay
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
              let left = updated.maxMisses - updated.wrong->Belt.Array.length
              let shown = letter->Js.String2.toLowerCase
              setNotice(_ => I18n.notice(uiLang, shown, left))
            }
            if updated.status == "won" {
              celebrate()
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
  let retryGame = () => startRound("/game/retry")

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

  let dragged = React.useRef("")

  // the physical keyboard listener mounts once, so route events through a ref
  // that always points at the latest render's handler
  let handleKeyRef = React.useRef(handleKey)
  handleKeyRef.current = handleKey

  React.useEffect0(() => {
    let listener = e =>
      if !(e->ctrlKey) && !(e->metaKey) && !(e->altKey) {
        handleKeyRef.current(e->eventKey)
      }
    addKeyListener("keydown", listener)
    Some(() => removeKeyListener("keydown", listener))
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
          <p className="error" role="alert"> {React.string(tr.serverWeak)} </p>
        </main>
  | Some(g) =>
    <main className="app">
      <header className="app-header">
        {
          // the flags choose the guessing direction, so they lock once the
          // round is under way — you can only switch on a fresh board
          let locked = g.guessed->Belt.Array.length > 0
          <div className="title-row">
            <button
              type_="button"
              className={g.direction == "it" ? "flag active" : "flag"}
              ariaLabel={tr.showItalian}
              disabled=locked
              onClick={_ => setDirection("it")->ignore}>
              {React.string(`🇮🇹`)}
            </button>
            <h1>
              {React.string("Le ")}
              <span className="cinque"> {React.string("Cinque")} </span>
            </h1>
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
        <div className="dateline">
          <span>
            {React.string(
              switch account {
              | Some(acc) => issueLabel(uiLang, acc.plays)
              | None => uiLang == #it ? "N. —" : "No. —"
              },
            )}
          </span>
          <span className="dateline-date"> {React.string(I18n.editionDate(uiLang))} </span>
        </div>
        <p className="tagline">
          <span className="tagline-text">
            // both languages are laid out in the same grid cell; the hidden one
            // still reserves space, so toggling never shifts the layout
            <span className={uiLang == #it ? "lang-line" : "lang-line hidden"}>
              {React.string(I18n.it.tagline)}
            </span>
            <span className={uiLang == #en ? "lang-line" : "lang-line hidden"}>
              {React.string(I18n.en.tagline)}
            </span>
          </span>
        </p>
        {switch account {
        | Some(acc) if !acc.guest =>
          // signed in: name opens a popup with the vocabulary count + log out
          <div className="account">
            <button
              type_="button"
              className="ghost username"
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
                    <button
                      type_="button"
                      className="ghost menu-logout"
                      onClick={_ => handleLogout()->ignore}>
                      {React.string(tr.logOut)}
                    </button>
                  </div>
                </>}
          </div>
        | _ =>
          // guest: a link to sign in and start tracking learned words
          <div className="account">
            <button type_="button" className="ghost username" onClick={_ => setShowAuth(_ => true)}>
              {React.string(tr.signIn)}
            </button>
          </div>
        }}
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
        <>
          <div className="pairs">
            {
              // the 🔊 pronounces the prompt word in its own language: Italian
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
                      onClick={_ => speakWord(p.prompt, promptLang)}>
                      {React.string(`🔊`)}
                    </button>
                    {React.string(p.prompt)}
                  </span>
                  <div className="english-tiles">
                    {p.tiles
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
          <div className="typed">
            <span className="typed-label"> {React.string(tr.typed)} </span>
            {g.guessed->Belt.Array.length == 0
              ? <span className="typed-empty"> {React.string(tr.noLettersYet)} </span>
              : g.guessed
                ->Belt.Array.mapWithIndex((i, l) =>
                  g.results->Belt.Array.get(i)->Belt.Option.getWithDefault(false)
                    ? <span
                        key={i->Belt.Int.toString}
                        className="chip hit"
                        style={{backgroundColor: tileColor(l)}}>
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
          {g.status == "playing"
            ? React.null
            : <div className="banner">
                <p> {React.string(g.status == "won" ? tr.wonBanner : tr.lostBanner)} </p>
                <div className="banner-actions">
                  {
                    // retrying the same words makes sense after a loss, not a win
                    g.status == "won"
                      ? React.null
                      : <button
                          type_="button"
                          className="ghost"
                          disabled=busy
                          onClick={_ => retryGame()->ignore}>
                          {React.string(tr.retry)}
                        </button>
                  }
                  <button
                    type_="button"
                    className="primary"
                    disabled=busy
                    onClick={_ => newGame()->ignore}>
                    {React.string(tr.newGame)}
                  </button>
                </div>
              </div>}
        </>
      }
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
      <footer className="app-footer"> {React.string(tr.footer)} </footer>
    </main>
  }
}
