import { api } from './client'
import type { Collection, Dataset, Label, Project } from '@/types'

export function listProjects() {
  return api<Project[]>('/api/projects')
}

export function createProject(name: string) {
  return api<Project>('/api/projects', { method: 'POST', body: JSON.stringify({ name }) })
}

export function getProject(projectId: number) {
  return api<Project>(`/api/projects/${projectId}`)
}

export function listCollections(projectId: number) {
  return api<Collection[]>(`/api/projects/${projectId}/collections`)
}

export function createCollection(projectId: number, name: string) {
  return api<Collection>(`/api/projects/${projectId}/collections`, {
    method: 'POST',
    body: JSON.stringify({ name }),
  })
}

export function getCollection(collectionId: number) {
  return api<Collection>(`/api/collections/${collectionId}`)
}

export function patchCollectionFieldSchema(collectionId: number, fieldSchemaJson: string) {
  return api<Collection>(`/api/collections/${collectionId}`, {
    method: 'PATCH',
    body: JSON.stringify({ fieldSchemaJson }),
  })
}

export function listLabels(projectId: number) {
  return api<Label[]>(`/api/projects/${projectId}/labels`)
}

export function createLabel(projectId: number, name: string) {
  return api<Label>(`/api/projects/${projectId}/labels`, {
    method: 'POST',
    body: JSON.stringify({ name }),
  })
}

export function listDatasets(projectId: number) {
  return api<Dataset[]>(`/api/projects/${projectId}/datasets`)
}

export interface CreateDatasetBody {
  name: string
  trainRatio: number
  validationRatio: number
  evaluationRatio: number
  seed?: number
  collectionIds?: number[]
  requireTranscription?: boolean
  silenceTrimRms?: number
  augmentNoiseDb?: number
  augmentMaxShiftMs?: number
  augmentVariantsPerClip?: number
}

export function createDataset(projectId: number, body: CreateDatasetBody) {
  return api<Dataset>(`/api/projects/${projectId}/datasets`, {
    method: 'POST',
    body: JSON.stringify(body),
  })
}
