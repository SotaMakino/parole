// Shared API client: distinguishes network failures, timeouts, and HTTP error statuses.

type apiError = {
  status: int, // 0 = network failure or timeout (no response at all)
  message: string,
}

type response
@get external ok: response => bool = "ok"
@get external status: response => int = "status"
@get external statusText: response => string = "statusText"
@send external json: response => promise<'a> = "json"

type abortSignal
@scope("AbortSignal") @val external timeoutSignal: int => abortSignal = "timeout"

type fetchOptions = {
  @as("method") method_: string,
  credentials: string,
  headers?: Js.Dict.t<string>,
  body?: string,
  signal: abortSignal,
}

@val external fetch: (string, fetchOptions) => promise<response> = "fetch"
@scope("JSON") @val external stringify: 'a => string = "stringify"

let api =
  (%raw(`import.meta.env.VITE_API_URL`): Js.Nullable.t<string>)
  ->Js.Nullable.toOption
  ->Belt.Option.getWithDefault("http://localhost:8080")

let timeoutMs = 8000

// The statuses this app can realistically hit, with human-readable hints.
let statusHint = (status, fallback) =>
  switch status {
  | 400 => "Bad Request — the server rejected the input"
  | 401 => "Unauthorized — you need to log in"
  | 403 => "Forbidden — logged in, but not allowed to do this"
  | 404 => "Not Found"
  | 409 => "Conflict — it already exists"
  | 500 => "Internal Server Error — something broke on the server"
  | 503 => "Service Unavailable — server is down or restarting"
  | _ => fallback
  }

let request = async (path, ~method_="GET", ~body: option<'a>=?) => {
  try {
    let options = switch body {
    | Some(b) => {
        method_,
        credentials: "include",
        headers: Js.Dict.fromArray([("Content-Type", "application/json")]),
        body: stringify(b),
        signal: timeoutSignal(timeoutMs),
      }
    | None => {method_, credentials: "include", signal: timeoutSignal(timeoutMs)}
    }
    let res = await fetch(api ++ path, options)
    if res->ok {
      Ok(res)
    } else {
      let data: Js.Json.t = try {await res->json} catch {
      | _ => Js.Json.null
      }
      let serverMessage =
        data
        ->Js.Json.decodeObject
        ->Belt.Option.flatMap(d => d->Js.Dict.get("error"))
        ->Belt.Option.flatMap(Js.Json.decodeString)
      let hint = switch serverMessage {
      | Some(msg) => msg
      | None => statusHint(res->status, res->statusText)
      }
      Error({
        status: res->status,
        message: `HTTP ${res->status->Belt.Int.toString}: ${hint}`,
      })
    }
  } catch {
  | Js.Exn.Error(e) if Js.Exn.name(e) == Some("TimeoutError") =>
    // a paused/frozen server never answers — without the timeout, fetch hangs for minutes
    Error({
      status: 0,
      message: `Server did not respond within ${(timeoutMs / 1000)->Belt.Int.toString}s.`,
    })
  | _ => Error({status: 0, message: "Cannot reach the server. Is it running?"})
  }
}
