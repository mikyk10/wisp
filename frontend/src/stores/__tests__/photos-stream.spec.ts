/**
 * Integration tests for usePhotosStore.loadPhotosStream.
 * These tests use a global fetch mock to simulate the NDJSON stream
 * without making real HTTP requests.
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { usePhotosStore } from '../photos'

vi.mock('@/api/photos', () => ({
  photosApi: { toggleVisibility: vi.fn().mockResolvedValue(undefined) },
}))

vi.mock('@/config', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/config')>()
  return {
    ...actual,
    API_BASE_URL: '',
    isApiMode: () => false,
    buildImageUrl: (_: string, id: number) => `https://picsum.photos/240/240?random=${id}`,
    getDataSourceUrl: (p: string) => `/mock-data/${p}`,
  }
})

// ---------- helpers ----------

function makeStream(...chunks: string[]): ReadableStream<Uint8Array> {
  const encoder = new TextEncoder()
  return new ReadableStream({
    start(controller) {
      for (const chunk of chunks) controller.enqueue(encoder.encode(chunk))
      controller.close()
    },
  })
}

function stubFetch(ndjson: string) {
  vi.stubGlobal(
    'fetch',
    vi.fn().mockResolvedValue({ ok: true, status: 200, body: makeStream(ndjson) })
  )
}

// ---------- tests ----------

describe('usePhotosStore — loadPhotosStream', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.unstubAllGlobals()
  })

  it('populates items with parsed photo records', async () => {
    stubFetch(
      '{"id":1,"enabled":true,"timestamp":"2024-06-15T00:00:00Z"}\n' +
        '{"id":2,"enabled":false,"timestamp":"2024-07-20T00:00:00Z"}\n'
    )

    const store = usePhotosStore()
    await store.loadPhotosStream('default')

    expect(store.items).toHaveLength(2)
    expect(store.items[0].id).toBe(1)
    expect(store.items[0].enabled).toBe(true)
    expect(store.items[1].id).toBe(2)
    expect(store.items[1].enabled).toBe(false)
  })

  it('sets streamCompleted = true and clears loading on success', async () => {
    stubFetch('{"id":1,"enabled":true,"timestamp":"2024-01-01T00:00:00Z"}\n')

    const store = usePhotosStore()
    await store.loadPhotosStream('default')

    expect(store.streamCompleted).toBe(true)
    expect(store.loading).toBe(false)
    expect(store.error).toBeNull()
  })

  it('builds the timeline grouped by year-month', async () => {
    stubFetch(
      '{"id":1,"enabled":true,"timestamp":"2024-06-15T00:00:00Z"}\n' +
        '{"id":2,"enabled":true,"timestamp":"2024-06-20T00:00:00Z"}\n' +
        '{"id":3,"enabled":true,"timestamp":"2023-12-01T00:00:00Z"}\n'
    )

    const store = usePhotosStore()
    await store.loadPhotosStream('default')

    const june = store.timelineEntries.find((e) => e.key === '2024-06')
    const dec = store.timelineEntries.find((e) => e.key === '2023-12')

    expect(june).toBeDefined()
    expect(june?.count).toBe(2)
    expect(june?.startIndex).toBe(0)
    expect(dec?.count).toBe(1)
    // newest-first ordering: 2024-06 must be index 0
    expect(store.timelineEntries[0].key).toBe('2024-06')
  })

  it('excludes photos with year < 1900 (Go zero value) from the timeline', async () => {
    stubFetch(
      '{"id":1,"enabled":true,"timestamp":"0001-01-01T00:00:00Z"}\n' +
        '{"id":2,"enabled":true,"timestamp":"2024-06-15T00:00:00Z"}\n'
    )

    const store = usePhotosStore()
    await store.loadPhotosStream('default')

    expect(store.items).toHaveLength(2) // both records are stored as photos
    expect(store.timelineEntries).toHaveLength(1) // only the valid date makes it into the timeline
    expect(store.timelineEntries[0].key).toBe('2024-06')
  })

  it('resets previous state at the start of a new load', async () => {
    stubFetch('{"id":1,"enabled":true,"timestamp":"2024-06-15T00:00:00Z"}\n')
    const store = usePhotosStore()
    await store.loadPhotosStream('default')
    expect(store.items).toHaveLength(1)

    stubFetch('{"id":9,"enabled":true,"timestamp":"2020-01-01T00:00:00Z"}\n')
    await store.loadPhotosStream('default')

    expect(store.items).toHaveLength(1)
    expect(store.items[0].id).toBe(9)
    expect(store.timelineEntries).toHaveLength(1)
    expect(store.timelineEntries[0].key).toBe('2020-01')
  })

  it('a catalog switch mid-stream discards the first stream entirely (generation guard)', async () => {
    // Catalog A: 60 records delivered immediately, stream kept open (never closes).
    const ndjsonA = Array.from({ length: 60 }, (_, i) =>
      JSON.stringify({ id: i + 1, enabled: true, timestamp: '2024-06-15T00:00:00Z' })
    ).join('\n') + '\n'
    const encoder = new TextEncoder()
    const openStream = new ReadableStream<Uint8Array>({
      start(controller) {
        controller.enqueue(encoder.encode(ndjsonA))
        // intentionally left open — only the abort (reader.cancel) ends it
      },
    })
    // Catalog B: 3 records, closes normally.
    const ndjsonB = [1001, 1002, 1003]
      .map((id) => JSON.stringify({ id, enabled: true, timestamp: '2023-03-03T00:00:00Z' }))
      .join('\n') + '\n'

    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce({ ok: true, status: 200, body: openStream })
      .mockResolvedValueOnce({ ok: true, status: 200, body: makeStream(ndjsonB) })
    vi.stubGlobal('fetch', fetchMock)

    const store = usePhotosStore()
    const first = store.loadPhotosStream('catalog-a')

    // Wait until the first batch of 50 is flushed and the stream is mid-flight.
    await vi.waitFor(() => expect(store.items.length).toBeGreaterThanOrEqual(50))

    const second = store.loadPhotosStream('catalog-b')
    await Promise.all([first, second])

    // Only catalog B's photos survive — no mixing from the aborted stream,
    // no post-loop flush, no timeline rebuilt from stale items.
    expect(store.items.map((p) => p.id)).toEqual([1001, 1002, 1003])
    expect(store.timelineEntries).toHaveLength(1)
    expect(store.timelineEntries[0].key).toBe('2023-03')
    expect(store.timelineEntries[0].count).toBe(3)
    // The aborted generation's finally must not clobber the new load's flags.
    expect(store.loading).toBe(false)
    expect(store.streamCompleted).toBe(true)
    expect(store.error).toBeNull()
  })

  it('an aborted stream does not set error or touch loading of the new generation', async () => {
    // Catalog A: slow stream that fails after the switch would have happened.
    let rejectRead!: (err: Error) => void
    const failingStream = new ReadableStream<Uint8Array>({
      pull() {
        return new Promise<void>((_, reject) => {
          rejectRead = reject
        })
      },
    })
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce({ ok: true, status: 200, body: failingStream })
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        body: makeStream('{"id":7,"enabled":true,"timestamp":"2024-01-01T00:00:00Z"}\n'),
      })
    vi.stubGlobal('fetch', fetchMock)

    const store = usePhotosStore()
    const first = store.loadPhotosStream('catalog-a')
    await Promise.resolve() // let the first fetch resolve and the read start

    const second = store.loadPhotosStream('catalog-b')
    await second
    rejectRead(new Error('boom from stale stream'))
    await first

    expect(store.error).toBeNull()
    expect(store.loading).toBe(false)
    expect(store.items.map((p) => p.id)).toEqual([7])
  })

  it('sets error state and clears loading when the HTTP response is not ok', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({ ok: false, status: 500, body: makeStream('') })
    )

    const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
    const store = usePhotosStore()
    await store.loadPhotosStream('default')
    consoleSpy.mockRestore()

    expect(store.error).toBeTruthy()
    expect(store.loading).toBe(false)
    expect(store.streamCompleted).toBe(false)
    expect(store.items).toHaveLength(0)
  })
})
