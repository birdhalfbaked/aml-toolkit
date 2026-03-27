<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import * as projectsApi from '@/api/projects'
import type { Collection, FieldDef, FieldSchema } from '@/types'
import { normalizedScope, parseFieldSchemaJson } from '@/types'
import { errorMessage } from '@/utils/error'

const props = defineProps<{
  modelValue: boolean
  collection: Collection | null
}>()

const emit = defineEmits<{
  'update:modelValue': [open: boolean]
  saved: []
}>()

const err = ref<string | null>(null)
const saving = ref(false)
const rows = ref<FieldDef[]>([])
const rawJson = ref('')
const jsonMode = ref(false)
const selectedIdx = ref<number>(0)
const showAdvanced = ref(false)

const selected = computed<FieldDef | null>(() => rows.value[selectedIdx.value] ?? null)

function displayScope(r: FieldDef) {
  return normalizedScope(r) === 'file' ? 'File' : 'Segment'
}

function displayType(r: FieldDef) {
  if (r.type === 'taxonomy') return 'Taxonomy'
  if (r.type === 'textarea') return 'Textarea'
  return 'Text'
}

watch(
  () => [props.modelValue, props.collection?.id, props.collection?.fieldSchemaJson] as const,
  () => {
    if (!props.modelValue || !props.collection) return
    const s = parseFieldSchemaJson(props.collection.fieldSchemaJson)
    rows.value = s.fields.map((f) => ({ ...f }))
    rawJson.value = JSON.stringify({ version: s.version, fields: s.fields }, null, 2)
    jsonMode.value = false
    err.value = null
    selectedIdx.value = 0
    showAdvanced.value = false
  },
)

function addRow() {
  rows.value.push({
    id: `field_${rows.value.length + 1}`,
    type: 'text',
    title: 'Field',
    required: false,
    scope: 'segment',
  })
  selectedIdx.value = Math.max(0, rows.value.length - 1)
  showAdvanced.value = true
}

function setRowScope(r: FieldDef, v: string) {
  r.scope = v === 'file' ? 'file' : 'segment'
  if (normalizedScope(r) === 'file' && r.type === 'taxonomy') {
    r.type = 'text'
  }
}

function typeSelectItems(r: FieldDef) {
  if (normalizedScope(r) === 'file') {
    return [
      { title: 'Text', value: 'text' },
      { title: 'Textarea', value: 'textarea' },
    ]
  }
  return [
    { title: 'Text', value: 'text' },
    { title: 'Textarea', value: 'textarea' },
    { title: 'Taxonomy (labels)', value: 'taxonomy' },
  ]
}

function removeRow(i: number) {
  rows.value.splice(i, 1)
  if (rows.value.length === 0) {
    selectedIdx.value = 0
    return
  }
  if (selectedIdx.value >= rows.value.length) selectedIdx.value = rows.value.length - 1
  if (selectedIdx.value === i) selectedIdx.value = Math.max(0, i - 1)
}

function close() {
  emit('update:modelValue', false)
}

function syncJsonFromRows() {
  const schema: FieldSchema = {
    version: 1,
    fields: rows.value.map((f) => ({
      ...f,
      id: (f.id ?? '').trim(),
      scope: normalizedScope(f),
    })),
  }
  rawJson.value = JSON.stringify(schema, null, 2)
}

function syncRowsFromJson(): boolean {
  try {
    const o = JSON.parse(rawJson.value) as FieldSchema
    if (!o.fields?.length) {
      err.value = 'Schema must include a non-empty fields array'
      return false
    }
    rows.value = o.fields.map((f) => ({ ...f }))
    selectedIdx.value = 0
    showAdvanced.value = false
    err.value = null
    return true
  } catch (e) {
    err.value = errorMessage(e)
    return false
  }
}

watch(
  jsonMode,
  (v) => {
    err.value = null
    if (v) {
      syncJsonFromRows()
    } else {
      syncRowsFromJson()
    }
  },
  { flush: 'sync' },
)

async function save() {
  if (!props.collection) return
  err.value = null
  saving.value = true
  try {
    let schema: FieldSchema
    if (jsonMode.value) {
      const o = JSON.parse(rawJson.value) as FieldSchema
      if (!o.fields?.length) {
        err.value = 'Schema must include a non-empty fields array'
        return
      }
      for (const f of o.fields) {
        const sc = f.scope === 'file' ? 'file' : 'segment'
        if (f.type === 'taxonomy' && sc !== 'segment') {
          err.value = `Taxonomy field "${f.id}" must be segment-scoped`
          return
        }
        if (sc === 'file' && f.type !== 'text' && f.type !== 'textarea') {
          err.value = `File-scoped field "${f.id}" must be text or textarea`
          return
        }
      }
      schema = { version: o.version || 1, fields: o.fields }
    } else {
      const ids = new Set<string>()
      for (const f of rows.value) {
        if (!f.id.trim()) {
          err.value = 'Each field needs an id'
          return
        }
        if (ids.has(f.id.trim())) {
          err.value = `Duplicate field id: ${f.id}`
          return
        }
        ids.add(f.id.trim())
      }
      for (const f of rows.value) {
        const sc = normalizedScope(f)
        if (f.type === 'taxonomy' && sc !== 'segment') {
          err.value = `Taxonomy field "${f.id}" must be segment-scoped`
          return
        }
        if (sc === 'file' && f.type !== 'text' && f.type !== 'textarea') {
          err.value = `File-scoped field "${f.id}" must be text or textarea`
          return
        }
      }
      schema = {
        version: 1,
        fields: rows.value.map((f) => ({
          ...f,
          id: f.id.trim(),
          scope: normalizedScope(f),
        })),
      }
    }
    await projectsApi.patchCollectionFieldSchema(props.collection.id, JSON.stringify(schema))
    emit('saved')
    close()
  } catch (e) {
    err.value = errorMessage(e)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <v-dialog :model-value="modelValue" max-width="900" @update:model-value="emit('update:modelValue', $event)">
    <v-card v-if="collection">
      <v-card-title class="d-flex align-center">
        Fields required for labeling
        <v-spacer />
        <v-btn-toggle v-model="jsonMode" density="compact" variant="outlined" divided mandatory>
          <v-btn :value="false" size="small">Editor</v-btn>
          <v-btn :value="true" size="small">JSON</v-btn>
        </v-btn-toggle>
      </v-card-title>
      <v-card-subtitle>{{ collection.name }} · collection #{{ collection.id }}</v-card-subtitle>
      <v-card-text>
        <v-alert type="info" variant="tonal" density="compact" class="mb-2">
          Use <strong>Segment</strong> scope for per-segment labels/transcripts, and <strong>File</strong> scope for metadata that applies to the whole audio file.
        </v-alert>
        <v-alert v-if="err" type="error" class="mb-4" density="compact">{{ err }}</v-alert>
        <template v-if="!jsonMode">
          <div class="schema-editor">
            <div class="schema-left">
              <div class="d-flex align-center ga-2 mb-2">
                <div class="text-subtitle-2">Fields</div>
                <v-spacer />
                <v-btn size="small" variant="tonal" @click="addRow">Add</v-btn>
              </div>

              <v-list v-if="rows.length" density="compact" border rounded class="schema-list">
                <v-list-item
                  v-for="(r, i) in rows"
                  :key="i"
                  :active="i === selectedIdx"
                  @click="selectedIdx = i"
                >
                  <template #title>
                    <span class="d-flex align-center ga-2">
                      <span class="schema-title text-truncate">
                        {{ r.title || r.id || 'Untitled' }}
                      </span>
                      <span v-if="r.required" class="schema-required" title="Required">*</span>
                    </span>
                  </template>
                  <template #subtitle>
                    <span class="d-flex align-center flex-wrap ga-1">
                      <v-chip size="x-small" variant="tonal" color="info">{{ displayScope(r) }}</v-chip>
                      <v-chip size="x-small" variant="tonal" color="secondary">{{ displayType(r) }}</v-chip>
                      <span class="text-caption text-medium-emphasis text-truncate">id: {{ r.id }}</span>
                    </span>
                  </template>
                  <template #append>
                    <v-menu location="bottom end">
                      <template #activator="{ props: mp }">
                        <v-btn
                          v-bind="mp"
                          icon="mdi-dots-vertical"
                          size="x-small"
                          variant="text"
                          class="schema-row-menu"
                          aria-label="Field actions"
                        />
                      </template>
                      <v-list density="compact">
                        <v-list-item title="Delete field" @click="removeRow(i)" />
                      </v-list>
                    </v-menu>
                  </template>
                </v-list-item>
              </v-list>
              <v-alert v-else type="info" density="compact" variant="tonal">No fields yet. Add one to get started.</v-alert>

              <div class="text-caption text-medium-emphasis mt-2">
                Tip: changing IDs won’t migrate existing saved values.
              </div>
            </div>

            <div class="schema-right">
              <template v-if="selected">
                <div class="d-flex align-center ga-2 mb-3">
                  <div class="text-subtitle-2">Field details</div>
                  <v-spacer />
                  <v-chip v-if="selected.required" size="x-small" variant="tonal" color="warning">Required</v-chip>
                </div>

                <v-text-field v-model="selected.title" label="Title" density="comfortable" variant="outlined" class="mb-2" />

                <div class="d-flex flex-wrap ga-2">
                  <v-select
                    :model-value="selected.scope ?? 'segment'"
                    label="Scope"
                    :items="[
                      { title: 'Segment', value: 'segment' },
                      { title: 'File', value: 'file' },
                    ]"
                    density="comfortable"
                    variant="outlined"
                    class="flex-1-1-0"
                    @update:model-value="setRowScope(selected, $event as string)"
                  />
                  <v-select
                    v-model="selected.type"
                    label="Type"
                    :items="typeSelectItems(selected)"
                    density="comfortable"
                    variant="outlined"
                    class="flex-1-1-0"
                  />
                </div>

                <div class="d-flex align-center ga-3 mt-1">
                  <v-switch v-model="selected.required" label="Required" color="primary" hide-details />
                  <v-spacer />
                </div>

                <v-expansion-panels v-model="showAdvanced" class="mt-3" variant="accordion">
                  <v-expansion-panel>
                    <v-expansion-panel-title>Advanced</v-expansion-panel-title>
                    <v-expansion-panel-text>
                      <v-text-field
                        v-model="selected.id"
                        label="Id"
                        density="comfortable"
                        variant="outlined"
                        hint="Changing IDs won’t migrate existing saved values."
                        persistent-hint
                      />
                    </v-expansion-panel-text>
                  </v-expansion-panel>
                </v-expansion-panels>
              </template>
              <v-alert v-else type="info" density="compact" variant="tonal">Select a field to edit.</v-alert>
            </div>
          </div>
        </template>
        <v-textarea v-else v-model="rawJson" rows="16" variant="outlined" auto-grow />
      </v-card-text>
      <v-card-actions>
        <v-btn variant="text" @click="close">Cancel</v-btn>
        <v-spacer />
        <v-btn color="primary" :loading="saving" @click="save">Save</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<style scoped>
.schema-editor {
  display: grid;
  grid-template-columns: 320px 1fr;
  gap: 16px;
  align-items: start;
}

.schema-list {
  max-height: 420px;
  overflow-y: auto;
}

.schema-row-menu {
  opacity: 0;
  transition: opacity 120ms ease;
}

:deep(.v-list-item:hover) .schema-row-menu {
  opacity: 1;
}

.schema-title {
  max-width: 220px;
}

.schema-required {
  color: rgb(var(--v-theme-error));
  font-weight: 700;
}

@media (max-width: 720px) {
  .schema-editor {
    grid-template-columns: 1fr;
  }
  .schema-title {
    max-width: 100%;
  }
}
</style>
