// Spell the play count for the masthead issue line (e.g. 130 → "centotrenta" /
// "one hundred thirty"), covering 0–9999; larger counts fall back to digits only.

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
