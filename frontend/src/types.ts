export interface Project {
  id: number
  name: string
  createdAt: string
}

export type FieldScope = 'segment' | 'file'

export interface FieldDef {
  id: string
  type: 'text' | 'textarea' | 'taxonomy'
  title: string
  required: boolean
  /** Default segment (per-clip); file = whole audio file */
  scope?: FieldScope
}

export interface FieldSchema {
  version: number
  fields: FieldDef[]
}

export interface Collection {
  id: number
  projectId: number
  name: string
  createdAt: string
  fieldSchemaJson?: string
}

export interface AudioFile {
  id: number
  collectionId: number
  storedFilename: string
  originalName: string
  format: string
  durationMs?: number
  uploadedAt: string
  fieldValues?: Record<string, string>
}

export interface Label {
  id: number
  projectId: number
  name: string
}

/** Client-only id for an unsaved segment drawn on the waveform; never returned by the API. */
export const DRAFT_SEGMENT_ID = -1

export interface Segment {
  id: number
  audioFileId: number
  startMs: number
  endMs: number
  labelId?: number
  transcription?: string
  labelName?: string
  fieldValues?: Record<string, string>
}

export interface LabelingQueueItem {
  audioFileId: number
  reason: string
}

export interface Dataset {
  id: number
  projectId: number
  name: string
  createdAt: string
  optionsJson: string
  storageRoot: string
}

export interface DatasetSample {
  id: number
  datasetId: number
  split: string
  filename: string
  relPath: string
  label: string
  transcription?: string
  sourceSegmentId: number
  augmentationJson?: string
}

export function normalizedScope(f: FieldDef): FieldScope {
  return f.scope === 'file' ? 'file' : 'segment'
}

export function defaultFieldSchema(): FieldSchema {
  return {
    version: 1,
    fields: [
      { id: 'label', type: 'taxonomy', title: 'Label', required: true, scope: 'segment' },
      { id: 'transcription', type: 'textarea', title: 'Transcription', required: false, scope: 'segment' },
    ],
  }
}

export function parseFieldSchemaJson(json: string | undefined | null): FieldSchema {
  if (!json || !json.trim()) return defaultFieldSchema()
  try {
    const o = JSON.parse(json) as FieldSchema
    if (!o.fields || !Array.isArray(o.fields) || o.fields.length === 0) return defaultFieldSchema()
    for (const f of o.fields) {
      if (!f.scope) f.scope = 'segment'
    }
    return o
  } catch {
    return defaultFieldSchema()
  }
}

/** Required segment-scoped fields non-empty in the form map. */
export function segmentFormComplete(schema: FieldSchema, fieldValues: Record<string, string>): boolean {
  for (const f of schema.fields) {
    if (normalizedScope(f) !== 'segment' || !f.required) continue
    if ((fieldValues[f.id] ?? '').trim() === '') return false
  }
  return true
}

/** Required file-scoped fields non-empty. */
export function fileFormComplete(schema: FieldSchema, fieldValues: Record<string, string>): boolean {
  for (const f of schema.fields) {
    if (normalizedScope(f) !== 'file' || !f.required) continue
    if ((fieldValues[f.id] ?? '').trim() === '') return false
  }
  return true
}
