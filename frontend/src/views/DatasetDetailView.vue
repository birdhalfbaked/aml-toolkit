<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useRoute } from 'vue-router'
import * as datasetsApi from '@/api/datasets'
import ProjectBreadcrumbs from '@/components/ProjectBreadcrumbs.vue'
import type { DatasetSample } from '@/types'
import { errorMessage } from '@/utils/error'

const route = useRoute()
const projectId = computed(() => Number(route.params.projectId))
const datasetId = computed(() => Number(route.params.datasetId))

const samples = ref<DatasetSample[]>([])
const err = ref<string | null>(null)
const selected = ref<DatasetSample | null>(null)
const playing = ref(false)
const audioUrl = computed(() =>
  selected.value ? datasetsApi.sampleAudioUrl(datasetId.value, selected.value.id) : '',
)

async function load() {
  err.value = null
  try {
    samples.value = await datasetsApi.listSamples(datasetId.value)
  } catch (e) {
    err.value = errorMessage(e)
  }
}

onMounted(load)

watch([projectId, datasetId], () => {
  selected.value = null
  load()
})

watch(selected, () => {
  playing.value = false
})

function onPlay() {
  playing.value = true
}
function onPause() {
  playing.value = false
}
function onEnded() {
  playing.value = false
}

async function downloadZip() {
  try {
    const blob = await datasetsApi.downloadDatasetZip(datasetId.value)
    const u = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = u
    a.download = `dataset_${datasetId.value}.zip`
    a.click()
    URL.revokeObjectURL(u)
  } catch (e) {
    err.value = errorMessage(e)
  }
}

const filterSplit = ref<string>('all')

const splitCounts = computed(() => {
  const c: Record<string, number> = { train: 0, validation: 0, evaluation: 0 }
  for (const s of samples.value) {
    if (s.split in c) c[s.split]++
  }
  return c
})

const augmentationSteps = computed(() => {
  if (!selected.value?.augmentationJson) return null
  try {
    const x = JSON.parse(selected.value.augmentationJson) as unknown
    if (!Array.isArray(x)) return null
    return x as Array<Record<string, any>>
  } catch {
    return null
  }
})

const filtered = computed(() => {
  if (filterSplit.value === 'all') return samples.value
  return samples.value.filter((s) => s.split === filterSplit.value)
})
</script>

<template>
  <div>
    <ProjectBreadcrumbs
      :project-id="projectId"
      :trail="[
        { title: 'Datasets', to: { name: 'datasets', params: { projectId: String(projectId) } } },
        { title: 'Dataset', disabled: true },
      ]"
    />
    <div class="d-flex align-center mb-4">
      <h1 class="text-h5">Dataset #{{ datasetId }}</h1>
      <v-spacer />
      <v-btn color="primary" variant="tonal" @click="downloadZip">Download ZIP</v-btn>
    </div>
    <v-alert v-if="err" type="error" class="mb-4">{{ err }}</v-alert>

    <v-select
      v-model="filterSplit"
      :items="[
        { title: 'All', value: 'all' },
        { title: 'train', value: 'train' },
        { title: 'validation', value: 'validation' },
        { title: 'evaluation', value: 'evaluation' },
      ]"
      item-title="title"
      item-value="value"
      label="Split"
      density="compact"
      hide-details
      class="mb-4"
      style="max-width: 220px"
    />
    <div class="d-flex flex-wrap ga-2 mb-4">
      <v-chip size="small" variant="tonal">train: {{ splitCounts.train }}</v-chip>
      <v-chip size="small" variant="tonal">validation: {{ splitCounts.validation }}</v-chip>
      <v-chip size="small" variant="tonal">evaluation: {{ splitCounts.evaluation }}</v-chip>
    </div>

    <v-row>
      <v-col cols="12" md="7">
        <v-table density="comfortable" hover>
          <thead>
            <tr>
              <th>Split</th>
              <th>Label</th>
              <th>File</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="s in filtered"
              :key="s.id"
              :class="{ 'bg-primary': selected?.id === s.id }"
              @click="selected = s"
            >
              <td>{{ s.split }}</td>
              <td>{{ s.label }}</td>
              <td class="text-truncate" style="max-width: 220px">{{ s.filename }}</td>
            </tr>
          </tbody>
        </v-table>
      </v-col>
      <v-col cols="12" md="5">
        <v-card v-if="selected" :class="{ 'border-active': playing }" variant="outlined" class="pa-4">
          <audio
            :key="audioUrl"
            controls
            class="w-100 mb-4"
            :src="audioUrl"
            @play="onPlay"
            @pause="onPause"
            @ended="onEnded"
          />
          <div class="text-subtitle-2 text-medium-emphasis">Label</div>
          <div class="text-h6 mb-2" :class="{ 'text-primary': playing }">{{ selected.label }}</div>
          <div class="text-subtitle-2 text-medium-emphasis">Transcription</div>
          <div class="text-body-1" :class="{ 'text-primary': playing }">{{ selected.transcription || '-' }}</div>
          <div v-if="augmentationSteps" class="mt-3">
            <div class="text-subtitle-2 text-medium-emphasis mb-1">Augmentation</div>
            <div v-for="(st, i) in augmentationSteps" :key="i" class="text-caption">
              <template v-if="st.type === 'shift'">
                shift: {{ st.shiftMs }}ms (frames={{ st.shiftFrames }})
              </template>
              <template v-else-if="st.type === 'noise'">
                noise: rms={{ (st.noiseRms ?? '').toString().slice(0, 8) }} ({{ st.distribution || 'gaussian' }})
              </template>
              <template v-else>
                {{ JSON.stringify(st) }}
              </template>
            </div>
          </div>
          <div v-else-if="selected.augmentationJson" class="text-caption mt-2">Augmentation: {{ selected.augmentationJson }}</div>
        </v-card>
        <v-alert v-else type="info" variant="tonal">Select a sample to play.</v-alert>
      </v-col>
    </v-row>
  </div>
</template>

<style scoped>
.border-active {
  border-color: rgb(var(--v-theme-primary)) !important;
  box-shadow: 0 0 0 1px rgb(var(--v-theme-primary));
}
</style>
