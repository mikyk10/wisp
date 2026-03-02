/// <reference types="vite/client" />

/**
 * Runtime environment injected by docker-entrypoint.sh (production)
 * or public/env.js (local dev, gitignored) via /env.js.
 */
interface Window {
  __env__?: {
    API_BASE_URL?: string
  }
}
