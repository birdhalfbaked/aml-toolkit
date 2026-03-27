import { api, apiBlob } from './client'
import type { Dataset, DatasetSample } from '@/types'

export function getDataset(id: number) {
  return api<Dataset>(`/api/datasets/${id}`)
}

export function listSamples(datasetId: number, split?: string) {
  const q = split ? `?split=${encodeURIComponent(split)}` : ''
  return api<DatasetSample[]>(`/api/datasets/${datasetId}/samples${q}`)
}

export function sampleAudioUrl(datasetId: number, sampleId: number) {
  const base = import.meta.env.VITE_API_BASE ?? ''
  return `${base}/api/datasets/${datasetId}/samples/${sampleId}/audio`
}

export function downloadDatasetZip(datasetId: number) {
  return apiBlob(`/api/datasets/${datasetId}/download`)
}
