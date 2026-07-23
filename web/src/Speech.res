// Word pronunciation.
//
// English uses the browser's built-in speechSynthesis — its default voice
// (e.g. macOS "Samantha") is already natural and costs nothing.
//
// Italian is the tricky one. Chromium picks a good Italian voice, so it also
// uses speechSynthesis. Other browsers (Firefox, Safari) only expose the
// low-quality "compact" voice, so there Italian is fetched as natural Google
// Cloud TTS audio from our backend's /tts endpoint and played through an
// <audio> element. The /tts endpoint is limited to signed-in accounts, so only
// authenticated non-Chromium users reach it; guests fall back to browser
// speech. Sending only that slice to the API keeps its usage (and cost) down.
// If the fetch fails (backend down or TTS unconfigured) we also fall back to
// browser speech, so a word still plays.

// --- browser speechSynthesis (English, and the Italian fallback) ---
type utterance
type voice
@new external makeUtterance: string => utterance = "SpeechSynthesisUtterance"
@set external setLang: (utterance, string) => unit = "lang"
@set external setRate: (utterance, float) => unit = "rate"
@set external setPitch: (utterance, float) => unit = "pitch"
@set external setVoice: (utterance, voice) => unit = "voice"
@get external voiceLang: voice => string = "lang"
@get external voiceDefault: voice => bool = "default"
@val @scope(("window", "speechSynthesis"))
external getVoices: unit => array<voice> = "getVoices"
@val @scope(("window", "speechSynthesis"))
external speak: utterance => unit = "speak"
@val @scope(("window", "speechSynthesis"))
external cancel: unit => unit = "cancel"

// Pin the OS default voice for a language when one is flagged (Chrome marks the
// enhanced voice default); otherwise let the browser choose, since overriding
// can pick a worse voice (e.g. forcing "Aaron" over Chrome's "Samantha").
let pickVoice = langCode => {
  let prefix = langCode->Js.String2.toLowerCase
  getVoices()->Js.Array2.find(v =>
    v->voiceDefault && v->voiceLang->Js.String2.toLowerCase->Js.String2.startsWith(prefix)
  )
}

let speakViaBrowser = (word, langCode) => {
  let u = makeUtterance(word->Js.String2.toLowerCase)
  u->setLang(langCode)
  switch pickVoice(langCode) {
  | Some(v) => u->setVoice(v)
  | None => ()
  }
  u->setRate(0.8) // slowed down so each syllable is easy to catch
  u->setPitch(1.0)
  speak(u)
}

// --- Google Cloud TTS audio (Italian) ---
type audio
@new external makeAudio: string => audio = "Audio"
@send external playAudio: audio => promise<unit> = "play"
@send external pauseAudio: audio => unit = "pause"
@set external setOnError: (audio, unit => unit) => unit = "onerror"
@val external encodeURIComponent: string => string = "encodeURIComponent"

// the audio currently playing, so a new click can cut it off
let current: ref<option<audio>> = ref(None)

let stopAudio = () =>
  switch current.contents {
  | Some(a) => a->pauseAudio
  | None => ()
  }

// play() can reject (e.g. autoplay policy); swallow it — a failed *load*
// triggers onerror separately, which is where the browser fallback lives.
let playSafely = async a =>
  try await a->playAudio catch {
  | _ => ()
  }

let isItalian = langCode => langCode->Js.String2.toLowerCase->Js.String2.startsWith("it")

// navigator.userAgentData exists only in Chromium (Firefox and Safari don't
// implement it), so its presence is a reliable "this browser has a good
// built-in Italian voice" signal — no UA-string sniffing needed.
let isChromium: bool = %raw(`typeof navigator !== "undefined" && navigator.userAgentData != null`)

// pronounce a word in the given BCP-47 voice ("it-IT" or "en-US").
// ~authenticated gates the Cloud TTS endpoint: guests can't call it, so they
// always get the browser voice.
let speakWord = (word, langCode, ~authenticated) => {
  cancel() // cut off any browser speech still playing
  stopAudio() // and any TTS audio still playing

  if isItalian(langCode) && !isChromium && authenticated {
    let w = word->Js.String2.toLowerCase
    let url = `${ApiClient.api}/tts?lang=it-IT&word=${encodeURIComponent(w)}`
    let a = makeAudio(url)
    current := Some(a)
    a->setOnError(() => speakViaBrowser(word, langCode)) // backend unavailable → browser voice
    playSafely(a)->ignore
  } else {
    speakViaBrowser(word, langCode)
  }
}
