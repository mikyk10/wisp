import { describe, it, expect } from 'vitest'
import { presentationFor, reasonTextFor } from '../deliveries'
import type { DeliveryKind, DeliveryReason } from '@/types'

describe('presentationFor', () => {
  it('names each known kind with a noun for what the server sent', () => {
    expect(presentationFor('photo').label).toBe('Photo')
    expect(presentationFor('http').label).toBe('HTTP image')
    expect(presentationFor('colorbar').label).toBe('Colour bar')
    expect(presentationFor('error').label).toBe('Error image')
  })

  it('groups colorbar and http into one visual family, apart from photo', () => {
    expect(presentationFor('colorbar').tone).toBe('generated')
    expect(presentationFor('http').tone).toBe('generated')
    expect(presentationFor('photo').tone).toBe('photo')
    expect(presentationFor('error').tone).toBe('error')
  })

  it('falls back for a kind added after this build', () => {
    const presentation = presentationFor('hologram' as DeliveryKind)
    expect(presentation.tone).toBe('unknown')
    expect(presentation.label).toBe('Unrecognised kind')
  })
})

describe('reasonTextFor', () => {
  it('renders every code in the closed set as a sentence', () => {
    const codes: DeliveryReason[] = [
      'no_images',
      'db_error',
      'file_missing',
      'no_catalog',
      'no_provider',
      'unknown_display',
      'load_failed',
      'encode_failed',
    ]

    for (const code of codes) {
      const text = reasonTextFor(code)
      expect(text).not.toBe('')
      // Prose, not the raw code echoed back at the reader.
      expect(text).not.toContain(code)
      expect(text.endsWith('.')).toBe(true)
    }
  })

  it('does not collapse load_failed into encode_failed', () => {
    // One sends you to the file, the other to the format.
    expect(reasonTextFor('load_failed')).not.toBe(reasonTextFor('encode_failed'))
  })

  it('returns nothing when there is no reason', () => {
    expect(reasonTextFor(null)).toBe('')
  })

  it('reports an unknown code as unknown, keeping the code itself', () => {
    const text = reasonTextFor('sunspots' as DeliveryReason)
    expect(text).toContain('not recognised')
    expect(text).toContain('sunspots')
  })
})
