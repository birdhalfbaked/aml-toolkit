import { api, apiBlob } from './client'
import type { Dataset, DatasetSample } from '@/types'

export function getDataset(id: number) {
  return api<Dataset>(`/api/datasets/${id}`)
}

export function listSamples(datasetId: number, split?: string) {
  const q = split ? `?split=${encodeURIComponent(split)}` : ''
  return api<DatasetSample[]>(`/api/datasets/${datasetId}/samples${q}`)
}

export function datasetSampleAudioPath(datasetId: number, sampleId: number) {
  return `/api/datasets/${datasetId}/samples/${sampleId}/audio`
}

export function sampleAudioUrl(datasetId: number, sampleId: number) {
  const base = import.meta.env.VITE_API_BASE ?? ''
  return `${base}${datasetSampleAudioPath(datasetId, sampleId)}`
}

export function downloadDatasetSampleAudio(datasetId: number, sampleId: number) {
  return apiBlob(datasetSampleAudioPath(datasetId, sampleId))
}

export function downloadDatasetZip(datasetId: number) {
  return apiBlob(`/api/datasets/${datasetId}/download`)
}
