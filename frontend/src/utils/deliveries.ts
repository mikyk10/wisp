import type { DeliveryKind, DeliveryReason } from '@/types'

/**
 * The vocabulary for one delivery kind, in one place.
 *
 * The strip and the expanded rows both name kinds, and the wording here is a
 * correctness matter rather than decoration: the server knows it handed over
 * bytes, never that a frame drew them. Keeping the strings together stops the
 * two views from drifting into different claims about the same record.
 */
export interface DeliveryKindPresentation {
  /** Human name for the kind, used in row headings and strip tooltips. */
  label: string
  /** mdi icon for the thumbnail placeholder. */
  icon: string
  /** Visual family — drives the strip glyph and the placeholder tint. */
  tone: 'photo' | 'generated' | 'error' | 'unknown'
}

const PRESENTATIONS: Record<DeliveryKind, DeliveryKindPresentation> = {
  photo: { label: 'Photo', icon: 'mdi-image-outline', tone: 'photo' },
  http: { label: 'HTTP image', icon: 'mdi-cloud-download-outline', tone: 'generated' },
  colorbar: { label: 'Colour bar', icon: 'mdi-palette-outline', tone: 'generated' },
  error: { label: 'Error image', icon: 'mdi-alert-circle-outline', tone: 'error' },
}

/** Fallback for a kind this build does not know about yet. */
const UNKNOWN: DeliveryKindPresentation = {
  label: 'Unrecognised kind',
  icon: 'mdi-help-circle-outline',
  tone: 'unknown',
}

/** Presentation for `kind`, falling back for kinds added after this build. */
export function presentationFor(kind: DeliveryKind): DeliveryKindPresentation {
  return PRESENTATIONS[kind] ?? UNKNOWN
}

/**
 * What each error reason means, in words.
 *
 * The server sends a code and leaves the prose to us, so this map is the whole
 * translation. `load_failed` and `encode_failed` are kept apart deliberately:
 * one sends you to look at the file, the other at the format, and collapsing
 * them into "the photo failed" would throw away the only thing that tells an
 * operator where to go next.
 */
const REASON_TEXT: Record<DeliveryReason, string> = {
  no_images: 'The catalog had no photos to show.',
  db_error: 'The catalog could not be read.',
  file_missing: 'The photo file was not found on disk.',
  no_catalog: 'No catalog is assigned to this display.',
  no_provider: 'The catalog type is not recognised.',
  unknown_display: 'This display is not configured.',
  load_failed: 'The photo could not be read.',
  encode_failed: 'The photo could not be converted for this panel.',
}

/**
 * The reason in words, or '' when there is none to give.
 *
 * A code this build has never heard of is reported as exactly that, with the
 * code itself kept for whoever goes looking in the server logs — inventing a
 * sentence for an unknown failure would describe something we cannot know.
 */
export function reasonTextFor(reason: DeliveryReason | null): string {
  if (reason === null) return ''
  return REASON_TEXT[reason] ?? `Reason not recognised by this build (${reason}).`
}
