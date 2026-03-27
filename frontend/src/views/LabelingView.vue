<script setup lang="ts">
import * as audioApi from '@/api/audio'
import * as projectsApi from '@/api/projects'
import AudioDropZone from '@/components/AudioDropZone.vue'
import SaveDatasetDialog from '@/components/SaveDatasetDialog.vue'
import CollectionSchemaDialog from '@/components/CollectionSchemaDialog.vue'
import ProjectBreadcrumbs from '@/components/ProjectBreadcrumbs.vue'
import SegmentFieldForm from '@/components/SegmentFieldForm.vue'
import WaveformEditor from '@/components/WaveformEditor.vue'
import { useLabelingQueue } from '@/composables/useLabelingQueue'
import { DRAFT_SEGMENT_ID, type AudioFile, type Collection, type Label, type Segment } from '@/types'
import {
  fileFormComplete,
  normalizedScope,
  parseFieldSchemaJson,
  segmentFormComplete,
} from '@/types'
import { errorMessage } from '@/utils/error'
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

const route = useRoute()
const router = useRouter()
const projectId = computed(() => Number(route.params.projectId))
const collectionId = computed(() => Number(route.params.collectionId))

const collection = ref<Collection | null>(null)
const fieldSchema = computed(() => parseFieldSchemaJson(collection.value?.fieldSchemaJson))

const segmentFields = computed(() => fieldSchema.value.fields.filter((f) => normalizedScope(f) === 'segment'))
const fileFields = computed(() => fieldSchema.value.fields.filter((f) => normalizedScope(f) === 'file'))

const files = ref<AudioFile[]>([])
const labels = ref<Label[]>([])
const segments = ref<Segment[]>([])
/** Bounds only; persisted on Save segment. Discarded when switching file, drawing a new region, or selecting a saved segment. */
const draftSegment = ref<{ startMs: number; endMs: number } | null>(null)
const selectedFileId = ref<number | null>(null)
const fieldValues = ref<Record<string, string>>({})
const fileFieldValues = ref<Record<string, string>>({})
const selectedSegId = ref<number | null>(null)
/** True after the user edits segment fields; cleared on segment change or successful save. */
const fieldDirty = ref(false)
/** True after the user edits file-level fields. */
const fileDirty = ref(false)
let syncingFields = false
let syncingFileFields = false
const err = ref<string | null>(null)
const saveDatasetOpen = ref(false)
const loopSegmentPlayback = ref(false)
/** Wide layout: hides upload + file list for more room on the waveform. */
const labelFocusMode = ref(false)
const uploadPhase = ref<'idle' | 'uploading' | 'processing'>('idle')
const uploadPercent = ref(0)
const schemaOpen = ref(false)

const waveRef = ref<{
  playSegmentRange: (startMs: number, endMs: number, loop: boolean) => Promise<void>
  stopSegmentPreview: () => void
  setZoom: (pxPerSec: number) => void
  resetZoom: () => void
  fitToView: () => void
} | null>(null)

const { queue, load: loadQueue, nextFileId } = useLabelingQueue(collectionId)

const selectedFile = computed(() => files.value.find((f) => f.id === selectedFileId.value))
const audioSrc = computed(() => (selectedFileId.value ? audioApi.audioUrl(selectedFileId.value) : ''))

const segmentRows = computed(() => {
  const taxFields = fieldSchema.value.fields.filter((f) => normalizedScope(f) === 'segment' && f.type === 'taxonomy')
  type Row = { id: number; startMs: number; endMs: number; hue: number; subtitle: string }
  const merged: { s: Segment; draft: boolean }[] = segments.value.map((s) => ({ s, draft: false }))
  if (draftSegment.value) {
    const d = draftSegment.value
    merged.push({
      s: {
        id: DRAFT_SEGMENT_ID,
        audioFileId: selectedFileId.value ?? 0,
        startMs: d.startMs,
        endMs: d.endMs,
      } as Segment,
      draft: true,
    })
  }
  merged.sort((a, b) => a.s.startMs - b.s.startMs)
  const rows: Row[] = merged.map((m) => {
    const s = m.s
    const hue = m.draft ? 43 : (merged.findIndex((x) => x.s.id === m.s.id) * 47) % 360
    const subtitle = m.draft
      ? 'Draft — not saved'
      : (() => {
          const fv = s.fieldValues ?? {}
          const vals = taxFields
            .map((f, idx) => {
              const v = (fv[f.id] ?? '').trim()
              if (v) return v
              if (idx === 0 && (s.labelName ?? '').trim()) return (s.labelName ?? '').trim()
              return ''
            })
            .filter(Boolean)
          return vals.length ? vals.join(' · ') : 'Unlabeled'
        })()
    return { id: s.id, startMs: s.startMs, endMs: s.endMs, hue, subtitle }
  })
  return rows
})

const segmentFormOk = computed(() => {
  if (selectedSegId.value == null) return false
  if (selectedSegId.value === DRAFT_SEGMENT_ID && !draftSegment.value) return false
  return segmentFormComplete(fieldSchema.value, fieldValues.value)
})

const fileFormOk = computed(() => {
  if (fileFields.value.length === 0) return true
  return fileFormComplete(fieldSchema.value, fileFieldValues.value)
})

const labelNames = computed(() => labels.value.map((l) => l.name))

async function loadFiles() {
  err.value = null
  try {
    collection.value = await projectsApi.getCollection(collectionId.value)
    files.value = await audioApi.listFiles(collectionId.value)
    labels.value = await projectsApi.listLabels(projectId.value)
    await loadQueue()
    if (!selectedFileId.value && files.value.length) {
      const n = nextFileId(null, files.value)
      selectedFileId.value = n ?? files.value[0].id
    }
  } catch (e) {
    err.value = errorMessage(e)
  }
}

async function loadSegments() {
  if (!selectedFileId.value) {
    segments.value = []
    return
  }
  segments.value = await audioApi.listSegments(selectedFileId.value)
  if (!fieldDirty.value) syncFieldValuesFromSelected()
}

async function refreshFileFieldsFromServer() {
  if (!selectedFileId.value) {
    fileFieldValues.value = {}
    return
  }
  syncingFileFields = true
  try {
    const a = await audioApi.getFile(selectedFileId.value)
    fileFieldValues.value = { ...(a.fieldValues ?? {}) }
    fileDirty.value = false
  } catch (e) {
    err.value = errorMessage(e)
  } finally {
    syncingFileFields = false
  }
}

watch(selectedFileId, async (id) => {
  waveRef.value?.stopSegmentPreview()
  draftSegment.value = null
  if (!id) {
    segments.value = []
    fileFieldValues.value = {}
    return
  }
  await loadSegments()
  await refreshFileFieldsFromServer()
  if (!fieldDirty.value) syncFieldValuesFromSelected()
})

watch(
  collectionId,
  () => {
    selectedFileId.value = null
    selectedSegId.value = null
    draftSegment.value = null
    loadFiles()
  },
  { immediate: true },
)

// Persist focus mode across sessions (migrate legacy "hide files" flag)
try {
  if (localStorage.getItem('labeling.labelFocusMode') === '1') {
    labelFocusMode.value = true
  } else if (localStorage.getItem('labeling.filesPanelCollapsed') === '1') {
    labelFocusMode.value = true
  }
} catch {}

watch(labelFocusMode, (v) => {
  try {
    localStorage.setItem('labeling.labelFocusMode', v ? '1' : '0')
  } catch {}
})

// Zoom is controlled via Ctrl + mousewheel on the waveform.

watch(
  fieldValues,
  () => {
    if (syncingFields) return
    fieldDirty.value = true
  },
  { deep: true },
)

watch(
  fileFieldValues,
  () => {
    if (syncingFileFields) return
    fileDirty.value = true
  },
  { deep: true },
)

function syncFieldValuesFromSelected() {
  const id = selectedSegId.value
  if (id === DRAFT_SEGMENT_ID) {
    return
  }
  const s = id ? segments.value.find((x) => x.id === id) : undefined
  syncingFields = true
  if (s) {
    const fv: Record<string, string> = { ...(s.fieldValues ?? {}) }
    const sch = fieldSchema.value
    const taxes = sch.fields.filter((f) => normalizedScope(f) === 'segment' && f.type === 'taxonomy')
    const primaryTax = taxes[0]
    if (primaryTax && !(fv[primaryTax.id] ?? '').trim() && s.labelName) {
      fv[primaryTax.id] = s.labelName
    }
    for (const f of sch.fields) {
      if (
        normalizedScope(f) === 'segment' &&
        f.type === 'textarea' &&
        !(fv[f.id] ?? '').trim() &&
        s.transcription
      ) {
        fv[f.id] = s.transcription
      }
    }
    fieldValues.value = fv
  } else {
    fieldValues.value = {}
  }
  syncingFields = false
}

watch(selectedSegId, () => {
  waveRef.value?.stopSegmentPreview()
  fieldDirty.value = false
  syncFieldValuesFromSelected()
})

watch(fieldSchema, () => {
  if (!fieldDirty.value) syncFieldValuesFromSelected()
})

async function onDrop(filesList: File[]) {
  err.value = null
  try {
    uploadPhase.value = 'uploading'
    uploadPercent.value = 0
    await audioApi.uploadToCollection(collectionId.value, filesList, {
      onPhase: (p) => {
        uploadPhase.value = p
      },
      onProgress: (p) => {
        uploadPercent.value = p.percent
      },
    })
    uploadPhase.value = 'idle'
    await loadFiles()
  } catch (e) {
    uploadPhase.value = 'idle'
    err.value = errorMessage(e)
  }
}

function selectFile(id: number) {
  selectedFileId.value = id
  selectedSegId.value = null
  draftSegment.value = null
}

function selectSegmentFromList(id: number) {
  if (id !== DRAFT_SEGMENT_ID) {
    draftSegment.value = null
  }
  selectedSegId.value = id
}

function onDraftChange(bounds: { startMs: number; endMs: number }) {
  fieldValues.value = {}
  fieldDirty.value = false
  draftSegment.value = bounds
  selectedSegId.value = DRAFT_SEGMENT_ID
}

async function onWaveRefresh() {
  await loadSegments()
  await loadQueue()
}

async function saveSegmentMeta() {
  if (selectedSegId.value == null) return
  err.value = null
  try {
    const schema = fieldSchema.value
    const fv: Record<string, string> = {}
    for (const f of schema.fields) {
      if (normalizedScope(f) === 'segment') {
        fv[f.id] = fieldValues.value[f.id] ?? ''
      }
    }
    let lid: number | null | undefined
    const taxes = schema.fields.filter((f) => normalizedScope(f) === 'segment' && f.type === 'taxonomy')
    const primaryTax = taxes[0]
    for (const t of taxes) {
      const name = (fv[t.id] ?? '').trim()
      if (!name) continue
      const existing = labels.value.find((l) => l.name === name)
      if (!existing) {
        await projectsApi.createLabel(projectId.value, name)
        labels.value = await projectsApi.listLabels(projectId.value)
      }
      if (primaryTax && t.id === primaryTax.id) {
        const now = labels.value.find((l) => l.name === name)
        lid = now?.id
      }
    }
    if (primaryTax && !(fv[primaryTax.id] ?? '').trim()) {
      lid = null
    }

    if (selectedSegId.value === DRAFT_SEGMENT_ID) {
      const d = draftSegment.value
      if (!d || !selectedFileId.value) return
      const created = await audioApi.createSegment(selectedFileId.value, {
        startMs: d.startMs,
        endMs: d.endMs,
        labelId: lid,
        fieldValues: fv,
      })
      draftSegment.value = null
      fieldDirty.value = false
      selectedSegId.value = created.id
      await loadSegments()
      await loadQueue()
      return
    }

    const s = segments.value.find((x) => x.id === selectedSegId.value)
    if (!s) return
    await audioApi.updateSegment(selectedSegId.value, {
      startMs: s.startMs,
      endMs: s.endMs,
      labelId: lid,
      fieldValues: fv,
    })
    fieldDirty.value = false
    await loadSegments()
    await loadQueue()
  } catch (e) {
    err.value = errorMessage(e)
  }
}

async function saveFileMeta() {
  if (!selectedFileId.value || fileFields.value.length === 0) return
  err.value = null
  try {
    const payload: Record<string, string> = {}
    for (const f of fileFields.value) {
      payload[f.id] = fileFieldValues.value[f.id] ?? ''
    }
    await audioApi.patchFile(selectedFileId.value, { fieldValues: payload })
    fileDirty.value = false
    await refreshFileFieldsFromServer()
    await loadQueue()
  } catch (e) {
    err.value = errorMessage(e)
  }
}

async function playSelectedSegment() {
  if (!waveRef.value) return
  if (selectedSegId.value === DRAFT_SEGMENT_ID && draftSegment.value) {
    const d = draftSegment.value
    await waveRef.value.playSegmentRange(d.startMs, d.endMs, loopSegmentPlayback.value)
    return
  }
  const s = segments.value.find((x) => x.id === selectedSegId.value)
  if (!s) return
  await waveRef.value.playSegmentRange(s.startMs, s.endMs, loopSegmentPlayback.value)
}

async function nextUnlabeled() {
  const n = nextFileId(selectedFileId.value, files.value)
  if (n) selectFile(n)
}

async function trimSel() {
  if (selectedSegId.value == null || selectedSegId.value === DRAFT_SEGMENT_ID) return
  err.value = null
  try {
    await audioApi.trimSilence(selectedSegId.value)
    await loadSegments()
  } catch (e) {
    err.value = errorMessage(e)
  }
}

async function deleteSel() {
  if (selectedSegId.value == null) return
  if (selectedSegId.value === DRAFT_SEGMENT_ID) {
    draftSegment.value = null
    selectedSegId.value = null
    return
  }
  err.value = null
  try {
    await audioApi.deleteSegment(selectedSegId.value)
    selectedSegId.value = null
    await loadSegments()
    await loadQueue()
  } catch (e) {
    err.value = errorMessage(e)
  }
}

async function deleteSelectedFile() {
  if (!selectedFileId.value) return
  const f = selectedFile.value
  const name = f?.originalName ?? `file #${selectedFileId.value}`
  if (!confirm(`Delete ${name}? This cannot be undone.`)) return
  err.value = null
  try {
    await audioApi.deleteFile(selectedFileId.value)
    selectedFileId.value = null
    selectedSegId.value = null
    segments.value = []
    await loadFiles()
  } catch (e) {
    err.value = errorMessage(e)
  }
}

function isTypingTarget(el: EventTarget | null): boolean {
  if (!el || !(el instanceof HTMLElement)) return false
  const tag = el.tagName
  if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return true
  if (el.isContentEditable) return true
  return el.closest('[contenteditable="true"]') != null
}

function onGlobalKeydown(e: KeyboardEvent) {
  if (e.key !== 'n' && e.key !== 'N') return
  if (e.ctrlKey || e.metaKey || e.altKey) return
  if (e.repeat) return
  if (isTypingTarget(e.target)) return
  e.preventDefault()
  void nextUnlabeled()
}

onMounted(() => {
  window.addEventListener('keydown', onGlobalKeydown)
})

onUnmounted(() => {
  window.removeEventListener('keydown', onGlobalKeydown)
})
</script>

<template>
  <div>
    <ProjectBreadcrumbs
      :project-id="projectId"
      :trail="[{ title: collection?.name ?? `Collection #${collectionId}`, disabled: true }]"
    />
    <div class="label-focus-hero d-flex align-center justify-center flex-wrap ga-3 mb-3 pa-4 rounded-lg">
      <v-tooltip location="bottom" max-width="300">
        <template #activator="{ props: tip }">
          <v-btn
            v-bind="tip"
            class="label-focus-hero-btn text-none font-weight-semibold"
            size="x-large"
            min-width="220"
            color="primary"
            :variant="labelFocusMode ? 'tonal' : 'flat'"
            :elevation="labelFocusMode ? 2 : 8"
            prepend-icon="mdi-image-filter-center-focus"
            rounded="pill"
            @click="labelFocusMode = !labelFocusMode"
          >
            {{ labelFocusMode ? 'Exit focus' : 'Label focus' }}
          </v-btn>
        </template>
        Wide layout for segment work: hides upload and the file list. Turn off when you need the file list or drop zone.
      </v-tooltip>
    </div>
    <div class="d-flex align-center flex-wrap ga-2 mb-2">
      <h1 class="text-h5">Labeling</h1>
      <v-tooltip v-if="queue.length" location="bottom" max-width="360">
        <template #activator="{ props: tip }">
          <v-chip v-bind="tip" size="small" color="warning" variant="tonal">{{ queue.length }} file(s) need labels</v-chip>
        </template>
        A file is queued if it has no segments, any segment is missing a required segment-level field, or required
        file-level fields are empty (per collection schema).
      </v-tooltip>
      <v-spacer />
      <v-tooltip location="bottom" text="Edit required fields (schema) for this collection">
        <template #activator="{ props: tip }">
          <v-btn v-bind="tip" size="small" variant="text" :disabled="!collection" @click="schemaOpen = true">
            Fields / schema
          </v-btn>
        </template>
      </v-tooltip>
      <v-btn
        v-if="selectedFileId"
        size="small"
        color="error"
        variant="text"
        @click="deleteSelectedFile"
      >
        Delete file
      </v-btn>
      <v-btn v-if="!labelFocusMode" size="small" variant="tonal" @click="nextUnlabeled">Next file</v-btn>
      <v-btn size="small" color="secondary" @click="saveDatasetOpen = true">Save as dataset</v-btn>
      <v-btn size="small" variant="text" :to="{ name: 'datasets', params: { projectId: String(projectId) } }">Datasets</v-btn>
    </div>
    <v-alert v-if="err" type="error" class="mb-4" closable @click:close="err = null">{{ err }}</v-alert>

    <v-sheet
      v-if="labelFocusMode && selectedFile"
      class="mb-3 pa-3 d-flex align-center flex-wrap ga-3 rounded"
      border
    >
      <div class="text-body-2 text-medium-emphasis text-truncate flex-grow-1 min-w-0" :title="selectedFile.originalName">
        {{ selectedFile.originalName }}
      </div>
      <v-btn color="primary" size="small" @click="nextUnlabeled">Next file</v-btn>
      <v-chip size="x-small" variant="outlined" class="text-medium-emphasis">N</v-chip>
    </v-sheet>

    <v-row>
      <v-col v-if="!labelFocusMode" cols="12" md="4">
        <v-sheet v-if="uploadPhase !== 'idle'" class="upload-status mb-2 pa-2 rounded" border>
          <div class="d-flex align-center ga-2">
            <div class="text-caption text-medium-emphasis">
              {{ uploadPhase === 'uploading' ? 'Uploading' : 'Processing ZIP' }}
            </div>
            <v-spacer />
            <div v-if="uploadPhase === 'uploading'" class="text-caption font-weight-medium">{{ uploadPercent }}%</div>
          </div>
          <v-progress-linear
            class="mt-1"
            color="primary"
            :model-value="uploadPhase === 'uploading' ? uploadPercent : undefined"
            :indeterminate="uploadPhase === 'processing'"
            height="6"
            rounded
          />
        </v-sheet>
        <AudioDropZone @files="onDrop" />
        <v-list v-if="files.length" class="mt-4" density="compact" border rounded>
          <v-list-item
            v-for="f in files"
            :key="f.id"
            :title="f.originalName"
            :subtitle="f.format + (f.durationMs ? ` · ${(f.durationMs / 1000).toFixed(1)}s` : '')"
            :active="f.id === selectedFileId"
            @click="selectFile(f.id)"
          />
        </v-list>
      </v-col>
      <v-col cols="12" :md="labelFocusMode ? 12 : 8">
        <template v-if="selectedFile && audioSrc">
          <v-card v-if="fileFields.length" class="mb-4" variant="outlined">
            <v-card-title class="text-subtitle-1">Whole file</v-card-title>
            <v-card-text>
              <SegmentFieldForm
                v-model="fileFieldValues"
                :fields="fileFields"
                :label-names="labelNames"
              />
              <v-btn
                color="primary"
                class="mt-2"
                size="small"
                :disabled="!fileFormOk || !fileDirty"
                @click="saveFileMeta"
              >
                Save file fields
              </v-btn>
            </v-card-text>
          </v-card>

          <WaveformEditor
            ref="waveRef"
            :audio-url="audioSrc"
            :segments="segments"
            :draft-segment="draftSegment"
            :selected-segment-id="selectedSegId"
            :snap-px="10"
            @refresh="onWaveRefresh"
            @select-segment="selectSegmentFromList"
            @draft-change="onDraftChange"
          />
          <div class="d-flex align-center ga-3 mt-2">
            <div class="text-caption text-medium-emphasis">Zoom: Ctrl + mousewheel (over waveform)</div>
            <v-spacer />
            <v-btn size="small" variant="text" @click="waveRef?.fitToView()">Fit</v-btn>
            <v-btn size="small" variant="text" @click="waveRef?.resetZoom()">Default</v-btn>
          </div>
          <div class="text-caption text-medium-emphasis mt-2">
            Drag on the waveform to mark a region (kept locally until Save segment). Drag edges to adjust. Unsaved regions
            are dropped if you switch files or draw again.
          </div>

          <v-row class="mt-4">
            <v-col cols="12" md="6">
              <div class="text-subtitle-2 mb-2">Segments</div>
              <v-list v-if="segmentRows.length" density="compact" border rounded class="segment-list">
                <v-list-item
                  v-for="row in segmentRows"
                  :key="row.id === DRAFT_SEGMENT_ID ? 'draft' : row.id"
                  :active="selectedSegId === row.id"
                  :title="
                    row.id === DRAFT_SEGMENT_ID
                      ? `Draft · ${row.startMs}–${row.endMs} ms`
                      : `#${row.id} · ${row.startMs}–${row.endMs} ms`
                  "
                  :subtitle="row.subtitle"
                  class="segment-list-item"
                  :style="{
                    borderLeft: `4px solid hsla(${row.hue}, 72%, 48%, 0.9)`,
                  }"
                  @click="selectSegmentFromList(row.id)"
                />
              </v-list>
              <v-alert v-else type="info" density="compact" variant="tonal">No segments yet — draw on the waveform.</v-alert>
            </v-col>
            <v-col cols="12" md="6">
              <div class="text-subtitle-2 mb-2">Segment fields</div>
              <template v-if="selectedSegId != null">
                <SegmentFieldForm
                  v-model="fieldValues"
                  :fields="segmentFields"
                  :label-names="labelNames"
                />
                <div class="d-flex flex-wrap align-center ga-2 mt-2">
                  <v-btn color="primary" size="small" :disabled="!segmentFormOk" @click="saveSegmentMeta">
                    Save segment
                  </v-btn>
                  <v-btn size="small" variant="tonal" @click="playSelectedSegment">Play segment</v-btn>
                  <v-checkbox
                    v-model="loopSegmentPlayback"
                    label="Loop"
                    hide-details
                    density="compact"
                  />
                  <v-btn
                    size="small"
                    variant="tonal"
                    :disabled="selectedSegId == null || selectedSegId === DRAFT_SEGMENT_ID"
                    @click="trimSel"
                  >
                    Trim silence
                  </v-btn>
                  <v-btn
                    size="small"
                    color="error"
                    variant="text"
                    :disabled="selectedSegId == null"
                    @click="deleteSel"
                  >
                    Delete
                  </v-btn>
                </div>
              </template>
              <v-alert v-else type="info" density="compact" variant="tonal">Select a segment to edit.</v-alert>
            </v-col>
          </v-row>
          <v-sheet class="pa-3 mt-4 rounded text-caption" border>
            <strong>Shortcut:</strong> N — next file in queue (ignored while typing in a field)
          </v-sheet>
        </template>
        <v-alert v-else type="info" variant="tonal">Select or upload an audio file.</v-alert>
      </v-col>
    </v-row>

    <SaveDatasetDialog
      v-model="saveDatasetOpen"
      :project-id="projectId"
      :collection-id="collectionId"
      @created="(e) => router.push({ name: 'dataset', params: { projectId: String(projectId), datasetId: String(e.id) } })"
    />
    <CollectionSchemaDialog v-model="schemaOpen" :collection="collection" @saved="loadFiles" />
  </div>
</template>

<style scoped>
.label-focus-hero {
  background: linear-gradient(
    180deg,
    rgba(var(--v-theme-primary), 0.12) 0%,
    rgba(var(--v-theme-surface), 0.4) 100%
  );
  border: 1px solid rgba(var(--v-theme-primary), 0.28);
  box-shadow: 0 1px 0 rgba(var(--v-theme-on-surface), 0.06);
}
.label-focus-hero-btn {
  letter-spacing: 0.02em;
}
.upload-status {
  background: rgb(var(--v-theme-surface));
}
.segment-list {
  max-height: 280px;
  overflow-y: auto;
}
.segment-list-item {
  border-left: 4px solid transparent;
}
</style>
