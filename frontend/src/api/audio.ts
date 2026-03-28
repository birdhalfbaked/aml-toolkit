import { api, uploadFiles, uploadFilesWithProgress, type UploadPhase, type UploadProgress } from './client'
import type { AudioFile, LabelingQueueItem, Segment } from '@/types'

export function listFiles(collectionId: number) {
  return api<AudioFile[]>(`/api/collections/${collectionId}/files`)
}

export function labelingQueue(collectionId: number) {
  return api<LabelingQueueItem[]>(`/api/collections/${collectionId}/labeling-queue`)
}

export function uploadToCollection(
  collectionId: number,
  files: File[],
  opts?: {
    onProgress?: (p: UploadProgress) => void
    onPhase?: (phase: UploadPhase) => void
  },
) {
  if (opts?.onProgress || opts?.onPhase) {
    return uploadFilesWithProgress(`/api/collections/${collectionId}/upload`, files, opts).then((res) => {
      if (!res.ok) throw new Error(res.text || res.statusText)
      return JSON.parse(res.text) as AudioFile[]
    })
  }
  return uploadFiles(`/api/collections/${collectionId}/upload`, files).then(async (res) => {
    const text = await res.text()
    if (!res.ok) throw new Error(text || res.statusText)
    return JSON.parse(text) as AudioFile[]
  })
}

export function audioUrl(fileId: number) {
  const base = import.meta.env.VITE_API_BASE ?? ''
  return `${base}/api/files/${fileId}/audio`
}

export function getFile(fileId: number) {
  return api<AudioFile>(`/api/files/${fileId}`)
}

export function patchFile(fileId: number, body: { fieldValues: Record<string, string> }) {
  return api<AudioFile>(`/api/files/${fileId}`, {
    method: 'PATCH',
    body: JSON.stringify(body),
  })
}

export function listSegments(fileId: number) {
  return api<Segment[]>(`/api/files/${fileId}/segments`)
}

export type SegmentWriteBody = {
  startMs: number
  endMs: number
  labelId?: number | null
  transcription?: string | null
  fieldValues?: Record<string, string>
}

export function createSegment(fileId: number, body: SegmentWriteBody) {
  return api<Segment>(`/api/files/${fileId}/segments`, {
    method: 'POST',
    body: JSON.stringify(body),
  })
}

export function updateSegment(id: number, body: SegmentWriteBody) {
  return api<{ ok: string }>(`/api/segments/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(body),
  })
}

export function deleteSegment(id: number) {
  return api<void>(`/api/segments/${id}`, { method: 'DELETE' })
}

export function deleteFile(fileId: number) {
  return api<void>(`/api/files/${fileId}`, { method: 'DELETE' })
}

export function trimSilence(segmentId: number, threshold?: number, windowMs?: number) {
  return api<{ startMs: number; endMs: number }>(`/api/segments/${segmentId}/trim-silence`, {
    method: 'POST',
    body: JSON.stringify({ threshold, windowMs }),
  })
}
