// Celebration fireworks: staggered bursts of randomized particles, plus the
// component that renders them. State (which bursts are live) stays in the caller.

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

let colors = ["#aa3bff", "#f59e0b", "#ef4444", "#22c55e", "#06b6d4", "#ec4899", "#facc15"]

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
      color: colors->Belt.Array.getExn(mod(i, Belt.Array.length(colors))),
      delay: Js.Math.random_int(0, 90),
      duration: 700 + Js.Math.random_int(0, 450),
      streak: mod(i, 3) == 0,
    }
  })
  {x, y, key, particles}
}

@react.component
let make = (~bursts: array<burst>) =>
  bursts
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
  ->React.array
