// All UI text in the two languages the masthead flags toggle between.
// "Le Cinque" (the paper's name) is never translated.
type t = {
  connecting: string,
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
  mistakes: string,
  wonBanner: string,
  lostBanner: array<string>, // one is shown at random when a round is lost
  sayings: array<string>, // Southern Italian proverbs; one shown under the lost banner
  newGame: string,
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
  agreePre: string,
  agreeAnd: string,
  privacyPolicy: string,
}

let it: t = {
  connecting: "Connessione al server…",
  serverWeak: "I nostri server sono un po' sovraccarichi. Riprova automatica tra 5 secondi.",
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
  mistakes: "Errori",
  wonBanner: "Bravo! Hai scoperto tutte e cinque le parole.",
  lostBanner: [
    "Cinque errori — riproviamo!",
    "Tentativi finiti — pronti per un altro round?",
    "Cinque errori. Ci riproviamo?",
    "Cinque scivoloni — dai, riprova!",
    "Niente più tentativi — riprovi?",
  ],
  sayings: [
    "Cu nesci, arrinesci.",
    "Cchiù scuru 'e mezzanotte nun pò venì.",
    "Ogni scarrafone è bello a mamma soja.",
    "Dicette 'o pappece vicino â noce: dammo tiempo ca te spertoso.",
    "Cu joca sulu 'un perdi mai.",
    "Chi tene 'a salute è ricco e nun 'o sape.",
    "Megghiu un tintu canusciutu ca un bonu a canusciri.",
    "Chi va cu' 'o zuoppo, 'mpara a zuppià.",
    "A lavà 'a capa ô ciuccio se perde acqua e sapone.",
    "Chi bella vò parè, guaje e pene adda patè.",
  ],
  newGame: "Nuova partita",
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
  agreePre: "Creando un account, accetti i nostri ",
  agreeAnd: " e l'",
  privacyPolicy: "Informativa sulla privacy",
}

let en: t = {
  connecting: "Connecting to server…",
  serverWeak: "Our servers are a bit overloaded right now. Retrying automatically in 5 seconds.",
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
  mistakes: "Mistakes",
  wonBanner: "Bravo! You revealed all five words.",
  lostBanner: [
    "Five mistakes — let's try again.",
    "Out of guesses — ready for another round?",
    "Five misses. Shall we go again?",
    "That's five slip-ups — give it another go.",
    "No more chances this round — try again?",
  ],
  sayings: [
    "Who leaves, succeeds.",
    "It can't get any darker than midnight.",
    "Every beetle is beautiful to its own mother.",
    "Said the weevil to the walnut: give me time and I'll bore right through you.",
    "Whoever plays alone never loses.",
    "Whoever has health is rich and doesn't know it.",
    "Better a known evil than an unknown good.",
    "Walk with the lame and you'll learn to limp.",
    "Washing a donkey's head only wastes the water and the soap.",
    "To look beautiful, you must suffer trouble and pain.",
  ],
  newGame: "New game",
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
  agreePre: "By creating an account, you agree to our ",
  agreeAnd: " and ",
  privacyPolicy: "Privacy Policy",
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
