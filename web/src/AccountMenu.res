// localized "24 Jul 2026" for a heatmap cell's tooltip; timeZone UTC keeps the
// date matching the calendar day regardless of the viewer's zone
@send
external toLocaleDate: (
  Js.Date.t,
  string,
  {"day": string, "month": string, "year": string, "timeZone": string},
) => string = "toLocaleDateString"

// the signed-in account popup: vocabulary count, progress bar, activity calendar,
// and the log-out / delete actions. The parent owns the open/closed state and
// mounts this only while the menu is open.
@react.component
let make = (
  ~lang,
  ~learned: int,
  ~activity: array<int>,
  ~activityStart: string,
  ~onClose: unit => unit,
  ~onLogout: unit => unit,
  ~onDelete: unit => unit,
) => {
  let tr = I18n.strings(lang)
  <>
    <div className="menu-backdrop" onClick={_ => onClose()} />
    <div className="account-menu" role="dialog">
      <div className="menu-stat">
        <span className="menu-count"> {React.string(learned->Belt.Int.toString)} </span>
        <span className="menu-label"> {React.string(tr.wordsLearned)} </span>
      </div>
      {
        // progress toward the 1,500-word course, capped at 100%
        let pct = learned * 100 / 1500
        let pct = pct > 100 ? 100 : pct
        <div className="menu-progress-wrap">
          <div className="menu-progress">
            <span style={{width: pct->Belt.Int.toString ++ "%"}} />
          </div>
          <p className="menu-progress-label">
            {React.string(
              learned->Belt.Int.toString ++
              " / " ++
              tr.wordGoal ++
              " · " ++
              pct->Belt.Int.toString ++ "%",
            )}
          </p>
        </div>
      }
      <div className="menu-cal-wrap">
        <span className="menu-label"> {React.string(tr.recentActivity)} </span>
        <div className="menu-cal">
          {
            // dense daily counts starting on a Sunday: chunk into
            // week columns, one cell per weekday (Sun→Sat)
            let cols = (Belt.Array.length(activity) + 6) / 7
            let locale = lang == #it ? "it-IT" : "en-US"
            let startMs = Js.Date.fromString(activityStart ++ "T00:00:00Z")->Js.Date.getTime
            Belt.Array.makeBy(cols, col =>
              <div className="cal-week" key={col->Belt.Int.toString}>
                {Belt.Array.makeBy(7, row => {
                  let i = col * 7 + row
                  switch Belt.Array.get(activity, i) {
                  | Some(c) =>
                    let lvl =
                      c <= 0 ? "0" : c <= 2 ? "1" : c <= 5 ? "2" : c <= 9 ? "3" : "4"
                    // "3 words · 24 Jul 2026" — the day's tally and date
                    let date = Js.Date.fromFloat(startMs +. i->Belt.Int.toFloat *. 86400000.)
                    let dateStr = date->toLocaleDate(
                      locale,
                      {
                        "day": "numeric",
                        "month": "short",
                        "year": "numeric",
                        "timeZone": "UTC",
                      },
                    )
                    let word = c == 1 ? tr.dayWord : tr.dayWords
                    let title = c->Belt.Int.toString ++ " " ++ word ++ " · " ++ dateStr
                    <div key={row->Belt.Int.toString} className={"cal-day l" ++ lvl} title />
                  | None =>
                    <div key={row->Belt.Int.toString} className="cal-day cal-empty" />
                  }
                })->React.array}
              </div>
            )->React.array
          }
        </div>
      </div>
      <div className="menu-actions">
        <button type_="button" className="link menu-logout" onClick={_ => onLogout()}>
          {React.string(tr.logOut)}
        </button>
        <button type_="button" className="link menu-delete" onClick={_ => onDelete()}>
          {React.string(tr.deleteAccount)}
        </button>
      </div>
    </div>
  </>
}
