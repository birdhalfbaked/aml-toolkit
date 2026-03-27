<script setup lang="ts">
import { ref } from 'vue'

const emit = defineEmits<{
  files: [files: File[]]
}>()

const inputEl = ref<HTMLInputElement | null>(null)

function onDrop(e: DragEvent) {
  e.preventDefault()
  const fl = e.dataTransfer?.files
  if (!fl?.length) return
  emit('files', Array.from(fl))
}

function onFile(e: Event) {
  const t = e.target as HTMLInputElement
  if (!t.files?.length) return
  emit('files', Array.from(t.files))
  t.value = ''
}

function pick() {
  inputEl.value?.click()
}
</script>

<template>
  <div class="drop pa-6 text-center" @dragover.prevent @drop="onDrop" @click="pick">
    <input
      ref="inputEl"
      type="file"
      multiple
      accept=".wav,.mp3,.zip,audio/*,application/zip"
      class="d-none"
      @change="onFile"
    />
    <v-icon size="48" class="mb-2">mdi-cloud-upload</v-icon>
    <div class="text-body-1">Drop MP3, WAV, or ZIP here - or click to choose files</div>
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
</style>
