<script setup lang="ts">
import { ref, computed } from 'vue'
import * as projectsApi from '@/api/projects'
import { errorMessage } from '@/utils/error'

const props = defineProps<{
  modelValue: boolean
  projectId: number
  collectionId?: number
}>()

const emit = defineEmits<{
  'update:modelValue': [v: boolean]
  created: [{ id: number }]
}>()

const name = ref('')
const train = ref(0.7)
const val = ref(0.15)
const ev = ref(0.15)
const requireTr = ref(false)
const trim = ref(false)
const noise = ref(0)
const shift = ref(0)
const variants = ref(0)
const busy = ref(false)
const err = ref<string | null>(null)

const sumOk = computed(() => Math.abs(train.value + val.value + ev.value - 1) < 0.001)

async function save() {
  err.value = null
  if (!name.value.trim()) {
    err.value = 'Name required'
    return
  }
  if (!sumOk.value) {
    err.value = 'Train + validation + evaluation must sum to 1'
    return
  }
  busy.value = true
  try {
    const body: projectsApi.CreateDatasetBody = {
      name: name.value.trim(),
      trainRatio: train.value,
      validationRatio: val.value,
      evaluationRatio: ev.value,
      requireTranscription: requireTr.value,
      augmentVariantsPerClip: variants.value,
    }
    if (props.collectionId) body.collectionIds = [props.collectionId]
    if (trim.value) body.silenceTrimRms = 0.02
    if (noise.value > 0) body.augmentNoiseDb = noise.value
    if (shift.value > 0) body.augmentMaxShiftMs = shift.value
    const ds = await projectsApi.createDataset(props.projectId, body)
    emit('created', { id: ds.id })
    emit('update:modelValue', false)
  } catch (e) {
    err.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <v-dialog :model-value="modelValue" max-width="520" @update:model-value="emit('update:modelValue', $event)">
    <v-card title="Save as dataset">
      <v-card-text>
        <v-alert v-if="err" type="error" class="mb-2" density="compact">{{ err }}</v-alert>
        <v-text-field v-model="name" label="Dataset name" density="comfortable" class="mb-2" />
        <div class="text-caption mb-1">Split ratios (must sum to 1)</div>
        <div class="d-flex ga-2 mb-2">
          <v-text-field v-model.number="train" type="number" label="Train" step="0.05" hide-details density="compact" />
          <v-text-field v-model.number="val" type="number" label="Validation" step="0.05" hide-details density="compact" />
          <v-text-field v-model.number="ev" type="number" label="Evaluation" step="0.05" hide-details density="compact" />
        </div>
        <v-switch v-model="requireTr" label="Require transcription on segments" color="primary" hide-details class="mb-1" />
        <v-switch v-model="trim" label="Apply silence trim (WAV) when materializing" color="primary" hide-details class="mb-1" />
        <v-text-field v-model.number="noise" type="number" label="Augment: noise (0–100, 0=off)" hide-details density="compact" class="mb-2" />
        <v-text-field v-model.number="shift" type="number" label="Augment: max shift ms (0=off)" hide-details density="compact" class="mb-2" />
        <v-text-field v-model.number="variants" type="number" label="Augment variants per clip (0–10)" hide-details density="compact" />
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="emit('update:modelValue', false)">Cancel</v-btn>
        <v-btn color="primary" :loading="busy" :disabled="!sumOk" @click="save">Save</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>
