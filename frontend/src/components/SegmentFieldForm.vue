<script setup lang="ts">
import type { FieldDef } from '@/types'

const props = defineProps<{
  fields: FieldDef[]
  modelValue: Record<string, string>
  labelNames: string[]
}>()

const emit = defineEmits<{
  'update:modelValue': [value: Record<string, string>]
}>()

function setField(id: string, v: string) {
  emit('update:modelValue', { ...props.modelValue, [id]: v })
}
</script>

<template>
  <div class="segment-field-form">
    <template v-for="f in fields" :key="f.id">
      <v-combobox
        v-if="f.type === 'taxonomy'"
        :model-value="modelValue[f.id] ?? ''"
        :items="labelNames"
        :label="f.title + (f.required ? ' *' : '')"
        density="comfortable"
        clearable
        hide-no-data
        class="mb-2"
        @update:model-value="(v) => setField(f.id, typeof v === 'string' ? v : String(v ?? ''))"
      />
      <v-textarea
        v-else-if="f.type === 'textarea'"
        :model-value="modelValue[f.id] ?? ''"
        :label="f.title + (f.required ? ' *' : '')"
        rows="2"
        density="comfortable"
        class="mb-2"
        @update:model-value="(v) => setField(f.id, v ?? '')"
      />
      <v-text-field
        v-else
        :model-value="modelValue[f.id] ?? ''"
        :label="f.title + (f.required ? ' *' : '')"
        density="comfortable"
        class="mb-2"
        @update:model-value="(v) => setField(f.id, v ?? '')"
      />
    </template>
  </div>
</template>
