import { describe, it, expect, vi, beforeEach } from 'vitest'
import { NDJSONStreamReader } from '../ndjsonStream'

// ---------- helpers ----------

/** Build a ReadableStream that yields the given chunks one at a time. */
function makeStream(...chunks: string[]): ReadableStream<Uint8Array> {
  const encoder = new TextEncoder()
  return new ReadableStream({
    start(controller) {
      for (const chunk of chunks) {
        controller.enqueue(encoder.encode(chunk))
      }
      controller.close()
    },
  })
}

/** Mock global.fetch to return the given stream body. */
function mockFetch(stream: ReadableStream<Uint8Array>, ok = true, status = 200) {
  vi.stubGlobal(
    'fetch',
    vi.fn().mockResolvedValue({
      ok,
      status,
      body: stream,
    })
  )
}

beforeEach(() => {
  vi.unstubAllGlobals()
})

// ---------- tests ----------

describe('NDJSONStreamReader', () => {
  describe('readStream — happy path', () => {
    it('yields a single complete record', async () => {
      mockFetch(makeStream('{"id":1,"enabled":true}\n'))

      const reader = new NDJSONStreamReader()
      const results: unknown[] = []
      for await (const item of reader.readStream('photos.ndjson')) {
        results.push(item)
      }

      expect(results).toHaveLength(1)
      expect(results[0]).toEqual({ id: 1, enabled: true })
    })

    it('yields multiple records from a single chunk', async () => {
      mockFetch(makeStream('{"id":1}\n{"id":2}\n{"id":3}\n'))

      const reader = new NDJSONStreamReader()
      const ids: unknown[] = []
      for await (const item of reader.readStream('test')) {
        ids.push((item as { id: number }).id)
      }

      expect(ids).toEqual([1, 2, 3])
    })

    it('handles records split across multiple chunks', async () => {
      // '{"id":1}' is split across two chunks
      mockFetch(makeStream('{"id":', '1}\n{"id":2}\n'))

      const reader = new NDJSONStreamReader()
      const results: unknown[] = []
      for await (const item of reader.readStream('test')) {
        results.push(item)
      }

      expect(results).toHaveLength(2)
      expect(results[0]).toEqual({ id: 1 })
      expect(results[1]).toEqual({ id: 2 })
    })

    it('handles a record with no trailing newline (flushed at end)', async () => {
      mockFetch(makeStream('{"id":42}')) // no trailing newline

      const reader = new NDJSONStreamReader()
      const results: unknown[] = []
      for await (const item of reader.readStream('test')) {
        results.push(item)
      }

      expect(results).toHaveLength(1)
      expect(results[0]).toEqual({ id: 42 })
    })

    it('skips blank lines without crashing', async () => {
      mockFetch(makeStream('{"id":1}\n\n{"id":2}\n'))

      const reader = new NDJSONStreamReader()
      const results: unknown[] = []
      for await (const item of reader.readStream('test')) {
        results.push(item)
      }

      expect(results).toHaveLength(2)
    })

    it('applies the type parameter so callers get typed items', async () => {
      interface Rec {
        id: number
        enabled: boolean
      }
      mockFetch(makeStream('{"id":7,"enabled":false}\n'))

      const reader = new NDJSONStreamReader<Rec>()
      const results: Rec[] = []
      for await (const item of reader.readStream('test')) {
        results.push(item)
      }

      // TypeScript would error here if item were unknown
      expect(results[0].id).toBe(7)
      expect(results[0].enabled).toBe(false)
    })
  })

  describe('readStream — error handling', () => {
    it('skips malformed JSON lines and continues', async () => {
      mockFetch(makeStream('{"id":1}\nNOT_JSON\n{"id":3}\n'))

      const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})
      const reader = new NDJSONStreamReader()
      const results: unknown[] = []
      for await (const item of reader.readStream('test')) {
        results.push(item)
      }

      expect(results).toHaveLength(2) // bad line skipped
      expect(warnSpy).toHaveBeenCalled()
      warnSpy.mockRestore()
    })

    it('throws when the HTTP response is not ok', async () => {
      mockFetch(makeStream(''), false, 404)

      const reader = new NDJSONStreamReader()
      await expect(async () => {
        // eslint-disable-next-line @typescript-eslint/no-unused-vars
        for await (const _item of reader.readStream('test')) {
          // should throw before yielding
        }
      }).rejects.toThrow('HTTP error! status: 404')
    })

    it('throws when response body is null', async () => {
      vi.stubGlobal(
        'fetch',
        vi.fn().mockResolvedValue({ ok: true, status: 200, body: null })
      )

      const reader = new NDJSONStreamReader()
      await expect(async () => {
        // eslint-disable-next-line @typescript-eslint/no-unused-vars
        for await (const _item of reader.readStream('test')) {
          // should throw before yielding
        }
      }).rejects.toThrow('Response body is not readable')
    })
  })
})
