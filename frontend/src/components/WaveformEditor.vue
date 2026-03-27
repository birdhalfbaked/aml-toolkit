<script setup lang="ts">
import * as audioApi from '@/api/audio'
import { DRAFT_SEGMENT_ID, type Segment } from '@/types'
import { onUnmounted, ref, shallowRef, watch } from 'vue'
import WaveSurfer from 'wavesurfer.js'
import type { Region } from 'wavesurfer.js/dist/plugins/regions.js'
import RegionsPlugin from 'wavesurfer.js/plugins/regions'

const props = defineProps<{
  audioUrl: string
  segments: Segment[]
  /** Unsaved region (frontend-only until Save segment). */
  draftSegment: { startMs: number; endMs: number } | null
  /** Highlights the active region; use DRAFT_SEGMENT_ID for the draft. */
  selectedSegmentId?: number | null
  /** Snap threshold in pixels at current zoom. */
  snapPx?: number
}>()

const emit = defineEmits<{
  refresh: []
  'select-segment': [id: number]
  'draft-change': [bounds: { startMs: number; endMs: number }]
}>()

const wrap = ref<HTMLElement | null>(null)
const ruler = ref<HTMLCanvasElement | null>(null)
const container = ref<HTMLElement | null>(null)
const ws = shallowRef<WaveSurfer | null>(null)
const regions = shallowRef<ReturnType<typeof RegionsPlugin.create> | null>(null)
let disableDragSelect: (() => void) | null = null
let unsubTime: (() => void) | null = null
/** Unsubscribers for per-region update-end listeners */
const regionUnsubs: Array<() => void> = []
let wheelListener: ((e: WheelEvent) => void) | null = null
let manualZoomForFile = false
let rafRuler: number | null = null

const rangeLoop = ref(false)
const rangeStartSec = ref<number | null>(null)
const rangeEndSec = ref<number | null>(null)

// Zoom handling (WaveSurfer zoom == minPxPerSec)
const defaultPxPerSec = 60
const pxPerSec = ref(defaultPxPerSec)
let pendingZoom: number | null = null

const RULER_H = 28
const WAVE_H = 160

function formatTime(sec: number): string {
  if (!Number.isFinite(sec) || sec < 0) sec = 0
  if (sec >= 60) {
    const m = Math.floor(sec / 60)
    const s = sec % 60
    return `${m}:${s < 10 ? '0' : ''}${s.toFixed(1)}`
  }
  if (sec >= 10) return sec.toFixed(1)
  return sec.toFixed(2)
}

function niceTickStep(secPerPx: number): number {
  // Choose a step so labels are ~80–140px apart.
  const targetPx = 110
  const targetSec = secPerPx * targetPx
  const steps = [0.01, 0.02, 0.05, 0.1, 0.2, 0.5, 1, 2, 5, 10, 15, 30, 60, 120, 300]
  for (const s of steps) if (s >= targetSec) return s
  return 600
}

function requestRenderRuler() {
  if (rafRuler != null) return
  rafRuler = requestAnimationFrame(() => {
    rafRuler = null
    renderRuler()
  })
}

function renderRuler() {
  const w = ws.value
  const c = ruler.value
  if (!w || !c) return

  const wrapEl = w.getWrapper()
  const viewportCss = w.getWidth?.() ?? wrapEl.getBoundingClientRect().width
  if (!viewportCss || viewportCss <= 0) return

  const dpr = window.devicePixelRatio || 1
  const width = Math.max(1, Math.floor(viewportCss * dpr))
  const height = Math.max(1, Math.floor(RULER_H * dpr))
  if (c.width !== width) c.width = width
  if (c.height !== height) c.height = height
  c.style.height = `${RULER_H}px`

  const ctx = c.getContext('2d')
  if (!ctx) return

  ctx.clearRect(0, 0, width, height)
  ctx.fillStyle = 'rgba(0,0,0,0.55)'
  ctx.fillRect(0, 0, width, height)
  ctx.strokeStyle = 'rgba(255,255,255,0.14)'
  ctx.beginPath()
  ctx.moveTo(0, height - 0.5)
  ctx.lineTo(width, height - 0.5)
  ctx.stroke()

  const dur = w.getDuration()
  if (!dur || dur <= 0) return
  // Use the rendered waveform width to match WaveSurfer's actual time scale.
  const scrollWidthPx = wrapEl.scrollWidth || viewportCss
  const pps = scrollWidthPx / dur
  const secPerPx = pps > 0 ? 1 / pps : 1 / defaultPxPerSec
  const startSec = w.getScroll() / (pps || defaultPxPerSec)
  const endSec = startSec + viewportCss / (pps || defaultPxPerSec)

  const step = niceTickStep(secPerPx)
  const minor = step / 5

  ctx.font = `${12 * dpr}px system-ui, -apple-system, Segoe UI, Roboto, Arial, sans-serif`
  ctx.textBaseline = 'top'
  ctx.fillStyle = 'rgba(255,255,255,0.96)'

  const firstMinor = Math.floor(startSec / minor) * minor
  for (let t = firstMinor; t <= endSec + minor; t += minor) {
    const xCss = (t - startSec) * (pps || defaultPxPerSec)
    const x = Math.round(xCss * dpr) + 0.5
    const isMajor = Math.abs(t / step - Math.round(t / step)) < 1e-6
    ctx.strokeStyle = isMajor ? 'rgba(255,255,255,0.85)' : 'rgba(255,255,255,0.45)'
    ctx.beginPath()
    ctx.moveTo(x, isMajor ? 0 : height * 0.35)
    ctx.lineTo(x, height)
    ctx.stroke()
    if (isMajor) {
      ctx.fillText(formatTime(t), x + 4 * dpr, 4 * dpr)
    }
  }
}

function computeAutoZoomPxPerSec(w: WaveSurfer): number {
  const dur = w.getDuration()
  if (!dur || dur <= 0) return defaultPxPerSec
  const wrap = w.getWrapper()
  const width = wrap?.getBoundingClientRect().width ?? 0
  const fitPx = width > 0 ? width / dur : 0

  const data = w.getDecodedData?.()
  const sr = (data as AudioBuffer | null)?.sampleRate ?? 0
  // Aim for a stable "samples per pixel" feel across files.
  const samplesPerPixel = 128
  let base = sr > 0 ? sr / samplesPerPixel : defaultPxPerSec

  // Boost short clips so they’re easier to inspect without immediate zooming.
  if (dur < 1) base *= 3
  else if (dur < 3) base *= 2

  const target = Math.max(fitPx, base)
  return Math.max(10, Math.min(5000, Math.round(target)))
}

/** Serialize segment API calls so SQLite is not hit with concurrent writes */
let apiChain: Promise<void> = Promise.resolve()
function enqueueSegmentApi(fn: () => Promise<void>) {
  apiChain = apiChain.then(fn).catch((e) => {
    console.error(e)
  })
}

function mergedSegments(): Segment[] {
  const out = props.segments.map((s) => ({ ...s }))
  if (props.draftSegment) {
    out.push({
      id: DRAFT_SEGMENT_ID,
      audioFileId: 0,
      startMs: props.draftSegment.startMs,
      endMs: props.draftSegment.endMs,
    } as Segment)
  }
  return out.sort((a, b) => a.startMs - b.startMs)
}

function hueForSegment(segId: number): number {
  if (segId === DRAFT_SEGMENT_ID) return 43
  const ordered = mergedSegments()
  const idx = ordered.findIndex((s) => s.id === segId)
  const i = idx >= 0 ? idx : 0
  return (i * 47) % 360
}

function regionColor(segId: number): string {
  const selected = props.selectedSegmentId === segId
  const alpha = selected ? 0.4 : 0.22
  return `hsla(${hueForSegment(segId)}, 72%, 48%, ${alpha})`
}

function snapMsFromPx(): number {
  const pps = pxPerSec.value > 0 ? pxPerSec.value : defaultPxPerSec
  const spx = props.snapPx ?? 10
  return (spx * 1000) / pps
}

function snapBounds(startMs: number, endMs: number, segId: number) {
  const snap = snapMsFromPx()
  let st = startMs
  let en = endMs
  const ordered = mergedSegments()
  const idx = ordered.findIndex((x) => x.id === segId)
  if (idx > 0) {
    const pe = ordered[idx - 1].endMs
    if (Math.abs(st - pe) <= snap) st = pe
  }
  if (idx >= 0 && idx < ordered.length - 1) {
    const ns = ordered[idx + 1].startMs
    if (Math.abs(en - ns) <= snap) en = ns
  }
  return { st, en }
}

function commitSegmentBounds(segId: number, startMs: number, endMs: number) {
  const { st, en } = snapBounds(startMs, endMs, segId)
  if (en <= st) return
  if (segId === DRAFT_SEGMENT_ID) {
    emit('draft-change', { startMs: st, endMs: en })
    return
  }
  enqueueSegmentApi(async () => {
    const s = props.segments.find((x) => x.id === segId)
    if (!s) return
    const body: audioApi.SegmentWriteBody = {
      startMs: st,
      endMs: en,
      labelId: s.labelId ?? null,
      transcription: s.transcription ?? null,
    }
    if (s.fieldValues && Object.keys(s.fieldValues).length > 0) {
      body.fieldValues = { ...s.fieldValues }
    }
    await audioApi.updateSegment(segId, body)
    emit('refresh')
  })
}

function syncRegionsFromProps() {
  const r = regions.value
  if (!r) return
  for (const u of regionUnsubs) u()
  regionUnsubs.length = 0
  r.clearRegions()
  for (const s of mergedSegments()) {
    const region = r.addRegion({
      id: String(s.id),
      start: s.startMs / 1000,
      end: s.endMs / 1000,
      color: regionColor(s.id),
      drag: true,
      resize: true,
    })
    const reg = region as Region
    const unsub = reg.on('update-end', () => {
      const id = Number(reg.id)
      if (!Number.isFinite(id)) return
      commitSegmentBounds(id, Math.round(reg.start * 1000), Math.round(reg.end * 1000))
    })
    regionUnsubs.push(unsub)
  }
}

/** Ignore region-created for regions added via addRegion (same id as server segment or draft). */
function isProgrammaticSegmentRegion(region: { id: string }): boolean {
  if (String(region.id) === String(DRAFT_SEGMENT_ID)) return true
  return props.segments.some((s) => String(s.id) === String(region.id))
}

/** Drag-selection can emit region-created multiple times; collapse to one API call */
let createDebounceTimer: ReturnType<typeof setTimeout> | null = null
let pendingCreate: { startMs: number; endMs: number } | null = null
const CREATE_DEBOUNCE_MS = 350
// No minimum segment duration; avoid accidental clicks via drag threshold instead.
const DRAG_SELECT_THRESHOLD_PX = 10

function scheduleDraftChange(startMs: number, endMs: number) {
  pendingCreate = { startMs, endMs }
  if (createDebounceTimer) clearTimeout(createDebounceTimer)
  createDebounceTimer = setTimeout(() => {
    createDebounceTimer = null
    const p = pendingCreate
    pendingCreate = null
    if (!p || p.endMs <= p.startMs) return
    emit('draft-change', { startMs: p.startMs, endMs: p.endMs })
  }, CREATE_DEBOUNCE_MS)
}

async function mountWs() {
  if (!container.value) return
  destroyWs()
  const reg = RegionsPlugin.create()
  const w = WaveSurfer.create({
    container: container.value,
    height: WAVE_H,
    waveColor: '#5c7cfa',
    progressColor: '#364fc7',
    url: props.audioUrl,
    minPxPerSec: pxPerSec.value,
    // Required for zoom to create a scrollable waveform.
    fillParent: false,
    plugins: [reg],
  })
  ws.value = w
  regions.value = reg

  unsubTime = w.on('timeupdate', (t) => {
    if (!rangeLoop.value || rangeStartSec.value == null || rangeEndSec.value == null) return
    if (t >= rangeEndSec.value - 0.025) {
      w.setTime(rangeStartSec.value)
    }
  })

  w.on('decode', () => {
    syncRegionsFromProps()
    requestRenderRuler()

    if (!manualZoomForFile) {
      const zAuto = computeAutoZoomPxPerSec(w)
      pendingZoom = zAuto
    }
    // Ensure zoom is applied after decode/render.
    const z = pendingZoom ?? pxPerSec.value
    pendingZoom = null
    pxPerSec.value = z
    w.zoom(z)
    requestRenderRuler()

    disableDragSelect = reg.enableDragSelection(
      {
      color: 'rgba(255, 193, 7, 0.3)',
      },
      DRAG_SELECT_THRESHOLD_PX,
    )

    w.on('scroll', () => requestRenderRuler())

    // Ctrl + mousewheel zoom (anchored under cursor).
    const el = wrap.value ?? w.getWrapper()
    wheelListener = (e: WheelEvent) => {
      if (!e.ctrlKey) return
      e.preventDefault()
      e.stopPropagation()
      const dur = w.getDuration()
      if (!dur || dur <= 0) return

      const rect = w.getWrapper().getBoundingClientRect()
      const x = Math.max(0, Math.min(rect.width, e.clientX - rect.left))
      const progress = rect.width > 0 ? x / rect.width : 0.5

      const beforePx = pxPerSec.value
      const beforeScrollPx = w.getScroll()
      const beforeScrollStartTime = beforeScrollPx / beforePx
      const beforeVisibleDur = rect.width / beforePx
      const anchorTime = beforeScrollStartTime + progress * beforeVisibleDur

      const zoomFactor = e.deltaY < 0 ? 1.15 : 1 / 1.15
      const nextPx = Math.max(10, Math.min(5000, Math.round(beforePx * zoomFactor)))
      if (nextPx === beforePx) return
      setZoom(nextPx, true)
      requestRenderRuler()

      const afterVisibleDur = rect.width / nextPx
      let newScrollStart = anchorTime - progress * afterVisibleDur
      if (newScrollStart < 0) newScrollStart = 0
      const maxStart = Math.max(0, dur - afterVisibleDur)
      if (newScrollStart > maxStart) newScrollStart = maxStart
      w.setScrollTime(newScrollStart)
    }
    el.addEventListener('wheel', wheelListener, { passive: false })
  })

  reg.on('region-clicked', (region) => {
    const id = Number(region.id)
    if (Number.isFinite(id)) emit('select-segment', id)
  })

  reg.on('region-created', (region) => {
    // addRegion() during sync also emits region-created - do not remove or POST those.
    if (isProgrammaticSegmentRegion(region)) {
      return
    }
    const startMs = Math.round(region.start * 1000)
    const endMs = Math.round(region.end * 1000)
    region.remove()
    scheduleDraftChange(startMs, endMs)
  })
}

function destroyWs() {
  if (disableDragSelect) {
    disableDragSelect()
    disableDragSelect = null
  }
  if (wheelListener && ws.value) {
    try {
      const el = wrap.value ?? ws.value.getWrapper()
      el.removeEventListener('wheel', wheelListener)
    } catch {}
  }
  wheelListener = null
  for (const u of regionUnsubs) u()
  regionUnsubs.length = 0
  unsubTime?.()
  unsubTime = null
  if (createDebounceTimer) {
    clearTimeout(createDebounceTimer)
    createDebounceTimer = null
  }
  pendingCreate = null
  rangeLoop.value = false
  rangeStartSec.value = null
  rangeEndSec.value = null
  if (rafRuler != null) {
    cancelAnimationFrame(rafRuler)
    rafRuler = null
  }
  ws.value?.destroy()
  ws.value = null
  regions.value = null
}

async function playSegmentRange(startMs: number, endMs: number, loop: boolean) {
  const w = ws.value
  if (!w) return
  const st = startMs / 1000
  const en = endMs / 1000
  rangeLoop.value = loop
  rangeStartSec.value = st
  rangeEndSec.value = en
  if (loop) {
    await w.play(st)
  } else {
    await w.play(st, en)
  }
}

function stopSegmentPreview() {
  rangeLoop.value = false
  rangeStartSec.value = null
  rangeEndSec.value = null
  ws.value?.pause()
}

function setZoom(newPxPerSec: number, manual: boolean) {
  if (manual) manualZoomForFile = true
  // Allow deep zoom-in; cap only to prevent runaway memory usage.
  const v = Math.max(10, Math.min(5000, Math.round(newPxPerSec)))
  pxPerSec.value = v
  if (ws.value) {
    ws.value.zoom(v)
    requestRenderRuler()
  } else {
    pendingZoom = v
  }
}

function resetZoom() {
  manualZoomForFile = false
  setZoom(defaultPxPerSec, false)
}

function fitToView() {
  const w = ws.value
  if (!w) return
  const dur = w.getDuration()
  if (!dur || dur <= 0) return
  const rect = w.getWrapper().getBoundingClientRect()
  const width = rect.width
  if (!width || width <= 0) return
  const fitPx = width / dur
  setZoom(fitPx, true)
}

defineExpose({
  playSegmentRange,
  stopSegmentPreview,
  setZoom: (v: number) => setZoom(v, true),
  resetZoom,
  fitToView,
  pxPerSec,
})

watch(
  () => props.audioUrl,
  () => {
    manualZoomForFile = false
    mountWs()
  },
  { immediate: true },
)

watch(
  () =>
    [
      props.selectedSegmentId,
      props.segments.map((s) => `${s.id}:${s.startMs}:${s.endMs}`).join('|'),
      props.draftSegment?.startMs,
      props.draftSegment?.endMs,
    ] as const,
  () => {
    if (regions.value) syncRegionsFromProps()
  },
)

onUnmounted(() => {
  destroyWs()
})
</script>

<template>
  <div ref="wrap" class="waveform-wrap">
    <canvas ref="ruler" class="ruler" />
    <div ref="container" class="waveform-root" />
  </div>
</template>

<style scoped>
.waveform-wrap {
  width: 100%;
  border-radius: 8px;
  overflow: hidden;
  background: rgb(var(--v-theme-surface-variant));
}
.ruler {
  width: 100%;
  height: 28px;
  display: block;
}
.waveform-root {
  min-height: 160px;
  width: 100%;
}
</style>
