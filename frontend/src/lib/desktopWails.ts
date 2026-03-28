import { OnFileDrop, OnFileDropOff } from '../../wailsjs/runtime/runtime'

/** True when running inside Wails with Go bindings (window.go.main.App). */
export function canUseDesktopNativeAudio(): boolean {
  if (typeof window === 'undefined') return false
  const w = window as unknown as { go?: { main?: { App?: unknown } } }
  return !!w.go?.main?.App
}

/** Alias for clarity where audio is not the focus (e.g. desktop file import). */
export const isWailsDesktop = canUseDesktopNativeAudio

function looksLikeJsonText(s: string): boolean {
  const t = s.trimStart()
  return t.startsWith('{') || t.startsWith('[')
}

/** Normalize URL-safe base64 and padding before atob. */
function normalizeBase64(s: string): string {
  let t = s.replace(/\s/g, '').replace(/-/g, '+').replace(/_/g, '/')
  const pad = t.length % 4
  if (pad) t += '='.repeat(4 - pad)
  return t
}

/** Decode base64 to bytes (Wails often passes Go []byte as base64 in bindings). */
function u8FromBase64(b64: string): Uint8Array {
  const bin = atob(normalizeBase64(b64))
  const out = new Uint8Array(bin.length)
  for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i) & 0xff
  return out
}

/** One byte per JS code unit (0–255); some bridges pass []byte this way. */
function u8FromLatin1CodeUnits(s: string): Uint8Array {
  const out = new Uint8Array(s.length)
  for (let i = 0; i < s.length; i++) out[i] = s.charCodeAt(i) & 0xff
  return out
}

function stringAllCodeUnitsInByteRange(s: string): boolean {
  for (let i = 0; i < s.length; i++) {
    if (s.charCodeAt(i) > 255) return false
  }
  return true
}

function u8FromWails(raw: unknown): Uint8Array {
  if (raw == null) return new Uint8Array()
  if (raw instanceof ArrayBuffer) return new Uint8Array(raw)
  if (ArrayBuffer.isView(raw)) {
    const v = raw as ArrayBufferView
    return new Uint8Array(v.buffer, v.byteOffset, v.byteLength)
  }
  if (Array.isArray(raw)) return new Uint8Array(raw as number[])
  if (typeof raw === 'string') {
    if (looksLikeJsonText(raw)) return new TextEncoder().encode(raw)
    try {
      return u8FromBase64(raw)
    } catch {
      if (stringAllCodeUnitsInByteRange(raw)) return u8FromLatin1CodeUnits(raw)
      return new TextEncoder().encode(raw)
    }
  }
  return new Uint8Array()
}

/** Wails class instances may hide fields from plain reads; Reflect + clone as fallback. */
function wailsObjectField<T>(raw: unknown, directKeys: string[], plainKeys: string[]): T | undefined {
  if (raw == null || typeof raw !== 'object') return undefined
  const o = raw as Record<string, unknown>
  const obj = o as object
  const keys = [...new Set([...directKeys, ...plainKeys])]
  for (const k of keys) {
    if (o[k] != null) return o[k] as T
  }
  for (const k of keys) {
    const v = Reflect.get(obj, k)
    if (v != null) return v as T
  }
  try {
    const plain = JSON.parse(JSON.stringify(raw)) as Record<string, unknown>
    for (const k of plainKeys) {
      if (plain[k] != null) return plain[k] as T
    }
  } catch {
    /* ignore — JSON.stringify can throw on huge []byte as number[] */
  }
  return undefined
}

/** In-process REST: same mux as the Wails asset server ([APIDispatchResult] from Go, not a JS tuple). */
export async function wailsApiDispatch(
  method: string,
  path: string,
  contentType: string,
  bodyText?: string,
): Promise<{ status: number; contentType: string; body: Uint8Array }> {
  if (!canUseDesktopNativeAudio()) {
    throw new Error('wailsApiDispatch: not in Wails')
  }
  const App = await import('../../wailsjs/go/main/App')
  const bytes = bodyText ? Array.from(new TextEncoder().encode(bodyText)) : []
  const raw: unknown = await (
    App as { ApiDispatch: (m: string, p: string, ct: string, b: number[]) => Promise<unknown> }
  ).ApiDispatch(method, path, contentType, bytes)
  if (!raw || typeof raw !== 'object') {
    throw new Error('ApiDispatch: unexpected return shape from Go')
  }
  const ro = raw as Record<string, unknown>
  const status = Number(ro.status ?? ro.Status ?? 0)
  const ct = String(ro.contentType ?? ro.ContentType ?? '')
  const bodyField = wailsObjectField<unknown>(raw, ['body', 'Body'], ['body', 'Body'])
  return { status, contentType: ct, body: u8FromWails(bodyField) }
}

/** Raw audio bytes for WaveSurfer (avoids WebView GET /api/files/:id/audio). */
export async function desktopReadAudioBlob(fileId: number): Promise<Blob> {
  if (!canUseDesktopNativeAudio()) {
    throw new Error('desktopReadAudioBlob: not in Wails')
  }
  const App = await import('../../wailsjs/go/main/App')
  const raw: unknown = await (
    App as { DesktopReadAudioFileForWaveform: (id: number) => Promise<unknown> }
  ).DesktopReadAudioFileForWaveform(fileId)
  if (!raw || typeof raw !== 'object') {
    throw new Error('DesktopReadAudioFileForWaveform: unexpected return shape from Go')
  }
  const dataRaw = wailsObjectField<unknown>(raw, ['data', 'Data'], ['data', 'Data'])
  const data = u8FromWails(dataRaw)
  const mimeStr = wailsObjectField<string>(raw, ['mime', 'Mime'], ['mime', 'Mime'])
  const mime = String(mimeStr ?? 'application/octet-stream')
  return new Blob([new Uint8Array(data)], { type: mime || 'application/octet-stream' })
}

/** Import absolute paths in Go (ZIP / WAV / MP3). Used for Wails file dialog and OS file drop. */
export async function desktopImportFromPaths(
  collectionId: number,
  paths: string[],
): Promise<import('@/types').AudioFile[]> {
  if (!paths.length || !canUseDesktopNativeAudio()) return []
  const App = await import('../../wailsjs/go/main/App')
  const rows = await App.DesktopImportFromPaths(collectionId, paths)
  return rows as unknown as import('@/types').AudioFile[]
}

/** OS folder picker for library root (desktop welcome). Returns '' if cancelled. */
export async function desktopOpenLibraryFolder(): Promise<string> {
  if (!canUseDesktopNativeAudio()) return ''
  const App = await import('../../wailsjs/go/main/App')
  const p = await App.DesktopOpenLibraryFolder()
  return typeof p === 'string' ? p : ''
}

/** Native OS file picker + import (ZIP / WAV / MP3). No-op / throws if not Wails. */
export async function desktopPickAndImportFiles(collectionId: number): Promise<
  import('@/types').AudioFile[]
> {
  if (!canUseDesktopNativeAudio()) return []
  const App = await import('../../wailsjs/go/main/App')
  const rows = await App.DesktopPickAndImportFiles(collectionId)
  return rows as unknown as import('@/types').AudioFile[]
}

/** Wails OS file drop → paths. Requires `--wails-drop-target: drop` on the drop zone. Cleanup on unmount. */
export function subscribeWailsFileDrop(
  handler: (paths: string[]) => void,
  useDropTarget = true,
): () => void {
  if (!canUseDesktopNativeAudio()) return () => {}
  OnFileDrop((_x, _y, paths) => {
    handler(paths)
  }, useDropTarget)
  return () => {
    OnFileDropOff()
  }
}

export async function desktopNativePlay(fileId: number, startMs: number): Promise<void> {
  if (!canUseDesktopNativeAudio()) return
  const App = await import('../../wailsjs/go/main/App')
  await App.DesktopAudioPlay(fileId, Math.round(startMs))
}

/** Seek the current desktop stream (file already playing via [desktopNativePlay]). */
export async function desktopNativeSeekMs(ms: number): Promise<void> {
  if (!canUseDesktopNativeAudio()) return
  const App = await import('../../wailsjs/go/main/App')
  await App.DesktopAudioSeekMs(Math.round(ms))
}

export async function desktopNativeStop(): Promise<void> {
  if (!canUseDesktopNativeAudio()) return
  const App = await import('../../wailsjs/go/main/App')
  await App.DesktopAudioStop()
}

/** Native save dialog + zip export; returns saved path or '' if cancelled. */
export async function desktopExportDatasetZip(datasetId: number): Promise<string> {
  if (!canUseDesktopNativeAudio()) return ''
  const App = await import('../../wailsjs/go/main/App')
  return App.DesktopExportDatasetZip(datasetId)
}

/** Subscribe to Go-emitted playback position in ms (~10 Hz while playing; also after seek). No-op in browser dev. */
export function subscribeDesktopAudioPosition(cb: (ms: number) => void): () => void {
  const w = window as unknown as {
    runtime?: { EventsOn?: (eventName: string, fn: (...args: unknown[]) => void) => () => void }
  }
  if (!w.runtime?.EventsOn) return () => {}
  return w.runtime.EventsOn('desktop:audio:position', (...args: unknown[]) => {
    const ms = Number(args[0])
    if (Number.isFinite(ms)) cb(ms)
  })
}
