<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import * as projectsApi from '@/api/projects'
import type { Project } from '@/types'
import type { Collection } from '@/types'
import CollectionSchemaDialog from '@/components/CollectionSchemaDialog.vue'
import ProjectBreadcrumbs from '@/components/ProjectBreadcrumbs.vue'
import { errorMessage } from '@/utils/error'

const route = useRoute()
const router = useRouter()
const projectId = computed(() => Number(route.params.projectId))

const project = ref<Project | null>(null)
const collections = ref<Collection[]>([])
const name = ref('')
const err = ref<string | null>(null)
const loading = ref(false)
const schemaOpen = ref(false)
const schemaCollection = ref<Collection | null>(null)

function openSchema(c: Collection, e: MouseEvent) {
  e.stopPropagation()
  schemaCollection.value = c
  schemaOpen.value = true
}

async function load() {
  loading.value = true
  err.value = null
  try {
    project.value = await projectsApi.getProject(projectId.value)
    collections.value = await projectsApi.listCollections(projectId.value)
  } catch (e) {
    err.value = errorMessage(e)
  } finally {
    loading.value = false
  }
}

async function create() {
  if (!name.value.trim()) return
  try {
    await projectsApi.createCollection(projectId.value, name.value.trim())
    name.value = ''
    await load()
  } catch (e) {
    err.value = errorMessage(e)
  }
}

onMounted(load)
</script>

<template>
  <div>
    <ProjectBreadcrumbs
      :project-id="projectId"
      :project-name="project?.name"
      project-link-disabled
    />
    <h1 class="text-h5 mb-4">Collections</h1>
    <v-alert v-if="err" type="error" class="mb-4">{{ err }}</v-alert>
    <v-row class="mb-4">
      <v-col cols="12" md="6">
        <v-text-field v-model="name" label="New collection name" density="comfortable" hide-details @keyup.enter="create" />
      </v-col>
      <v-col cols="auto">
        <v-btn color="primary" @click="create">Add collection</v-btn>
      </v-col>
      <v-col cols="auto">
        <v-btn variant="tonal" :to="{ name: 'datasets', params: { projectId: String(projectId) } }">Saved datasets</v-btn>
      </v-col>
    </v-row>
    <v-list v-if="collections.length" lines="two" border rounded>
      <v-list-item
        v-for="c in collections"
        :key="c.id"
        :title="c.name"
        :subtitle="`Collection #${c.id}`"
        @click="router.push({ name: 'labeling', params: { projectId: String(projectId), collectionId: String(c.id) } })"
      >
        <template #append>
          <v-tooltip location="bottom" text="Fields / schema">
            <template #activator="{ props: tip }">
              <v-btn
                v-bind="tip"
                icon="mdi-form-select"
                size="small"
                variant="text"
                aria-label="Edit fields / schema"
                @click="openSchema(c, $event)"
              />
            </template>
          </v-tooltip>
        </template>
      </v-list-item>
    </v-list>
    <v-alert v-else-if="!loading" type="info" variant="tonal">No collections — add one to upload audio.</v-alert>

    <CollectionSchemaDialog v-model="schemaOpen" :collection="schemaCollection" @saved="load" />
  </div>
</template>
