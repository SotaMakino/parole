// Cloudflare Pages Function: reverse-proxy every /api/* request to the Render
// backend. Because the browser only ever talks to the Pages origin, the session
// cookie the backend sets is first-party (SameSite=Lax), so it survives across
// sessions instead of being dropped as a cross-site cookie.
//
// Set the API_ORIGIN environment variable in the Pages project to the Render URL
// (e.g. https://le-cinque.onrender.com) — no trailing slash.
export async function onRequest({ request, env }) {
  const origin = env.API_ORIGIN;
  if (!origin) {
    return new Response(
      JSON.stringify({ error: "API_ORIGIN is not configured" }),
      { status: 500, headers: { "Content-Type": "application/json" } },
    );
  }

  const url = new URL(request.url);
  // strip the leading /api, forward the rest of the path plus the query string
  const target = origin + url.pathname.replace(/^\/api/, "") + url.search;

  // new Request(target, request) carries the method, headers (incl. Cookie) and
  // body over unchanged; returning new Response(body, resp) preserves the status
  // and headers (incl. Set-Cookie) coming back from the backend.
  const resp = await fetch(new Request(target, request));
  return new Response(resp.body, resp);
}
