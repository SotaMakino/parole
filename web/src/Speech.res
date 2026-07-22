// Browser text-to-speech for word pronunciation.

type utterance
@new external makeUtterance: string => utterance = "SpeechSynthesisUtterance"
@set external setLang: (utterance, string) => unit = "lang"
@set external setRate: (utterance, float) => unit = "rate"
@set external setPitch: (utterance, float) => unit = "pitch"
@val @scope(("window", "speechSynthesis"))
external speak: utterance => unit = "speak"
@val @scope(("window", "speechSynthesis"))
external cancel: unit => unit = "cancel"

// pronounce a word in the given BCP-47 voice ("it-IT" or "en-US")
let speakWord = (word, langCode) => {
  cancel() // cut off any word still playing
  let u = makeUtterance(word->Js.String2.toLowerCase)
  u->setLang(langCode)
  u->setRate(0.8) // slowed down so each syllable is easy to catch
  u->setPitch(1.0) // pitch
  speak(u)
}
