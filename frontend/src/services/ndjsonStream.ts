import { getDataSourceUrl } from '../config'

/**
 * Reads a newline-delimited JSON stream and yields each parsed object.
 *
 * @template T The expected shape of each JSON record.
 */
export class NDJSONStreamReader<T = unknown> {
  async *readStream(
    resourcePath: string,
    signal?: AbortSignal,
  ): AsyncGenerator<T, void, unknown> {
    const finalUrl = getDataSourceUrl(resourcePath)
    const response = await fetch(finalUrl, { signal })

    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`)
    }

    const reader = response.body?.getReader()
    if (!reader) {
      throw new Error('Response body is not readable')
    }

    // Cancel the reader when the signal fires so reader.read() rejects
    // even if the response body is already buffered locally.
    let released = false
    const onAbort = () => {
      if (!released) reader.cancel().catch(() => {})
    }
    if (signal) {
      signal.addEventListener('abort', onAbort, { once: true })
    }

    const decoder = new TextDecoder()
    let buffer = ''

    try {
      while (true) {
        if (signal?.aborted) return

        const { done, value } = await reader.read()

        if (done) {
          if (signal?.aborted) return
          // Flush any remaining buffered data
          if (buffer.trim()) {
            try {
              yield JSON.parse(buffer.trim()) as T
            } catch (error) {
              console.warn('Parse error on last line:', error, buffer)
            }
          }
          break
        }

        buffer += decoder.decode(value, { stream: true })
        const lines = buffer.split('\n')

        // Keep the last (potentially incomplete) chunk in the buffer
        buffer = lines.pop() ?? ''

        for (const line of lines) {
          if (signal?.aborted) return
          const trimmedLine = line.trim()
          if (trimmedLine) {
            try {
              yield JSON.parse(trimmedLine) as T
            } catch (error) {
              console.warn('Parse error on line:', error, trimmedLine)
            }
          }
        }
      }
    } finally {
      released = true
      signal?.removeEventListener('abort', onAbort)
      reader.releaseLock()
    }
  }
}
