const apiBase = import.meta.env.VITE_API_BASE ?? ''

export async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(apiBase + path, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      ...init?.headers,
    },
  })
  const text = await res.text()
  if (!res.ok) {
    let msg = text
    try {
      const j = JSON.parse(text) as { error?: string }
      if (j.error) msg = j.error
    } catch {
      /* ignore */
    }
    throw new Error(msg || res.statusText)
  }
  if (!text) return undefined as T
  return JSON.parse(text) as T
}

export function apiBlob(path: string): Promise<Blob> {
  return fetch(apiBase + path).then((res) => {
    if (!res.ok) throw new Error(res.statusText)
    return res.blob()
  })
}

export function uploadFiles(path: string, files: File[]): Promise<Response> {
  const fd = new FormData()
  for (const f of files) fd.append('files', f)
  return fetch(apiBase + path, { method: 'POST', body: fd })
}

export type UploadPhase = 'uploading' | 'processing'
export type UploadProgress = { loaded: number; total: number; percent: number }

export function uploadFilesWithProgress(
  path: string,
  files: File[],
  opts?: {
    onProgress?: (p: UploadProgress) => void
    onPhase?: (phase: UploadPhase) => void
  },
): Promise<{ ok: boolean; status: number; statusText: string; text: string }> {
  const fd = new FormData()
  for (const f of files) fd.append('files', f)

  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    xhr.open('POST', apiBase + path, true)

    xhr.upload.addEventListener('loadstart', () => {
      opts?.onPhase?.('uploading')
    })
    xhr.upload.addEventListener('progress', (e) => {
      const total = e.total || 0
      const loaded = e.loaded || 0
      const percent = total > 0 ? Math.round((loaded / total) * 100) : 0
      opts?.onProgress?.({ loaded, total, percent })
    })
    xhr.upload.addEventListener('loadend', () => {
      // Upload done; server may still be processing ZIP contents.
      opts?.onPhase?.('processing')
    })

    xhr.addEventListener('error', () => reject(new Error('upload failed')))
    xhr.addEventListener('abort', () => reject(new Error('upload aborted')))
    xhr.addEventListener('load', () => {
      resolve({
        ok: xhr.status >= 200 && xhr.status < 300,
        status: xhr.status,
        statusText: xhr.statusText,
        text: xhr.responseText ?? '',
      })
    })
    xhr.send(fd)
  })
}
