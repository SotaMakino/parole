import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// Inline the built CSS straight into the HTML so it costs zero render-blocking
// network requests. The stylesheet is a few KiB, so this is cheaper than a
// separate request on the critical path (Lighthouse: "render-blocking requests").
function inlineCss() {
  return {
    name: 'inline-css',
    enforce: 'post',
    apply: 'build',
    generateBundle(_, bundle) {
      const html = Object.values(bundle).filter((f) => f.fileName.endsWith('.html'))
      for (const page of html) {
        page.source = page.source.replace(
          /<link[^>]*rel="stylesheet"[^>]*href="([^"]+)"[^>]*>/g,
          (tag, href) => {
            const name = href.replace(/^\//, '')
            const css = bundle[name]
            if (!css) return tag
            delete bundle[name] // drop the now-unused .css file
            return `<style>${css.source}</style>`
          },
        )
      }
    },
  }
}

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), inlineCss()],
  test: {
    include: ['tests/**/*_test.res.mjs'],
    environment: 'happy-dom',
  },
})
