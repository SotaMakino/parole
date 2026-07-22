// All UI text in the two languages the masthead flags toggle between.
// "Le Cinque" (the paper's name) is never translated.
type t = {
  connecting: string,
  retry: string,
  serverWeak: string,
  signIn: string,
  account: string,
  wordsLearned: string,
  logOut: string,
  deleteAccount: string,
  deleteConfirm: string,
  close: string,
  showItalian: string,
  showEnglish: string,
  tagline: string,
  typed: string,
  noLettersYet: string,
  mistakes: string,
  wonBanner: string,
  lostBanner: string,
  newGame: string,
  footer: string,
  privacy: string,
  terms: string,
  logInTitle: string,
  signUpTitle: string,
  username: string,
  password: string,
  show: string,
  hide: string,
  logIn: string,
  createAccount: string,
  pleaseWait: string,
  noAccountQ: string,
  haveAccountQ: string,
}

let it: t = {
  connecting: "Connessione al server…",
  retry: "Riprova",
  serverWeak: "I nostri server fanno schifo. Riprova automatica tra 5 secondi.",
  signIn: "Accedi",
  account: "Account",
  wordsLearned: "parole imparate",
  logOut: "Esci",
  deleteAccount: "Elimina account",
  deleteConfirm: "Eliminare il tuo account e tutti i suoi dati? L'operazione è irreversibile.",
  close: "Chiudi",
  showItalian: "Mostra in italiano",
  showEnglish: "Mostra in inglese",
  tagline: "Scegli una lettera e mettila esattamente al suo posto — trascinala, oppure tocca la lettera e poi la casella",
  typed: "Digitate",
  noLettersYet: "ancora nessuna lettera",
  mistakes: "Errori",
  wonBanner: "Bravo! Hai scoperto tutte e cinque le parole.",
  lostBanner: "Cinque errori — partita finita. Studia le risposte; queste parole torneranno per il ripasso.",
  newGame: "Nuova partita",
  footer: "Fatto da un umano e da un'IA, ma solo l'umano fa errori.",
  privacy: "Privacy",
  terms: "Termini",
  logInTitle: "Accedi",
  signUpTitle: "Registrati",
  username: "Nome utente",
  password: "Password",
  show: "Mostra",
  hide: "Nascondi",
  logIn: "Accedi",
  createAccount: "Crea account",
  pleaseWait: "Attendere…",
  noAccountQ: "Non hai un account? ",
  haveAccountQ: "Hai già un account? ",
}

let en: t = {
  connecting: "Connecting to server…",
  retry: "Retry",
  serverWeak: "Our servers are shit-weak. It'll auto-retry in 5 seconds.",
  signIn: "Sign in",
  account: "Account",
  wordsLearned: "words learned",
  logOut: "Log out",
  deleteAccount: "Delete account",
  deleteConfirm: "Delete your account and all its data? This cannot be undone.",
  close: "Close",
  showItalian: "Show in Italian",
  showEnglish: "Show in English",
  tagline: "Pick a letter and place it on its exact spot — drag it, or tap the letter then the tile",
  typed: "Typed",
  noLettersYet: "no letters yet",
  mistakes: "Mistakes",
  wonBanner: "Bravo! You revealed all five words.",
  lostBanner: "Five mistakes — game over. Study the answers; these words will come back for review.",
  newGame: "New game",
  footer: "Made by a human and AI, but only the human makes mistakes.",
  privacy: "Privacy",
  terms: "Terms",
  logInTitle: "Log in",
  signUpTitle: "Sign up",
  username: "Username",
  password: "Password",
  show: "Show",
  hide: "Hide",
  logIn: "Log in",
  createAccount: "Create account",
  pleaseWait: "Please wait…",
  noAccountQ: "No account? ",
  haveAccountQ: "Already have an account? ",
}

let strings = lang => lang == #it ? it : en

// dynamic strings that fold in a value
let pronounce = (lang, word) => lang == #it ? `Pronuncia ${word}` : `Pronounce ${word}`

let notice = (lang, letter, left) =>
  if lang == #it {
    left > 0
      ? `Nessuna "${letter}" qui — ${left->Belt.Int.toString} ${left == 1
            ? "tentativo rimasto"
            : "tentativi rimasti"}.`
      : `Nessuna "${letter}" qui.`
  } else if left > 0 {
    `No "${letter}" there — ${left->Belt.Int.toString} ${left == 1 ? "try" : "tries"} left.`
  } else {
    `No "${letter}" there.`
  }

let failedLoad = (lang, msg) =>
  lang == #it ? `Impossibile caricare la partita: ${msg}` : `Failed to load the game: ${msg}`
let failedStart = (lang, msg) =>
  lang == #it
    ? `Impossibile iniziare una nuova partita: ${msg}`
    : `Failed to start a new game: ${msg}`
let failedSubmit = (lang, msg) =>
  lang == #it ? `Impossibile inviare la lettera: ${msg}` : `Failed to submit the letter: ${msg}`

// the dateline date, in the selected language's locale
let editionDateIt: string = %raw(`new Date().toLocaleDateString('it-IT', {
  weekday: 'long', day: 'numeric', month: 'long', year: 'numeric'
})`)
let editionDateEn: string = %raw(`new Date().toLocaleDateString('en-US', {
  weekday: 'long', day: 'numeric', month: 'long', year: 'numeric'
})`)
let editionDate = lang => lang == #it ? editionDateIt : editionDateEn
