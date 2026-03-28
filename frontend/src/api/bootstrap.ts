import { api } from './client'

export type BootstrapStatus = {
  needsOnboarding: boolean
  libraryRoot: string
  recommendedLibraryRoot: string
  stateDir: string
  databasePath: string
  desktopBootstrapEnabled: boolean
}

let cached: BootstrapStatus | null = null

export function clearBootstrapCache() {
  cached = null
}

export async function getBootstrapStatus(force = false): Promise<BootstrapStatus> {
  if (cached && !force) return cached
  cached = await api<BootstrapStatus>('/api/bootstrap/status')
  return cached
}

export async function completeBootstrap(libraryRoot: string): Promise<void> {
  await api<{ ok: string }>('/api/bootstrap/complete', {
    method: 'POST',
    body: JSON.stringify({ libraryRoot }),
  })
  clearBootstrapCache()
}
