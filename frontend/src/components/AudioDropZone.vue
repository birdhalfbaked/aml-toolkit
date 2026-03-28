<script setup lang="ts">
import { computed, ref } from 'vue'

const props = defineProps<{
  /** When true, skip web File upload; parent uses native paths (Wails dialog / OS file drop). */
  desktopImportOnly?: boolean
}>()

const emit = defineEmits<{
  files: [files: File[]]
  'pick-native': []
}>()

const inputEl = ref<HTMLInputElement | null>(null)

const dropClass = computed(() => ({
  drop: true,
  'pa-6': true,
  'text-center': true,
  'wails-file-drop-target': props.desktopImportOnly,
}))

function onDrop(e: DragEvent) {
  e.preventDefault()
  if (props.desktopImportOnly) return
  const fl = e.dataTransfer?.files
  if (!fl?.length) return
  emit('files', Array.from(fl))
}

function onFile(e: Event) {
  if (props.desktopImportOnly) return
  const t = e.target as HTMLInputElement
  if (!t.files?.length) return
  emit('files', Array.from(t.files))
  t.value = ''
}

function pick() {
  if (props.desktopImportOnly) {
    emit('pick-native')
    return
  }
  inputEl.value?.click()
}
</script>

<template>
  <div :class="dropClass" @dragover.prevent @drop="onDrop" @click="pick">
    <input
      v-if="!desktopImportOnly"
      ref="inputEl"
      type="file"
      multiple
      accept=".wav,.mp3,.zip,audio/*,application/zip"
      class="d-none"
      @change="onFile"
    />
    <v-icon size="48" class="mb-2">mdi-cloud-upload</v-icon>
    <div v-if="desktopImportOnly" class="text-body-1">
      Drop MP3, WAV, or ZIP from Explorer here — or click to choose files
    </div>
    <div v-else class="text-body-1">Drop MP3, WAV, or ZIP here - or click to choose files</div>
    <div class="text-caption text-medium-emphasis mt-1">ZIP: flattened WAV paths; loose files: MP3 + WAV</div>
  </div>
</template>

<style scoped>
.drop {
  border: 2px dashed rgba(var(--v-theme-outline), 0.5);
  border-radius: 12px;
  cursor: pointer;
  transition: background 0.15s;
}
.drop:hover {
  background: rgba(var(--v-theme-primary), 0.06);
}
/* Wails: only elements with this marker receive OS file paths via OnFileDrop. */
.wails-file-drop-target {
  --wails-drop-target: drop;
}
</style>
