<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import * as projectsApi from '@/api/projects'
import ProjectBreadcrumbs from '@/components/ProjectBreadcrumbs.vue'
import type { Dataset } from '@/types'
import { errorMessage } from '@/utils/error'

const route = useRoute()
const router = useRouter()
const projectId = computed(() => Number(route.params.projectId))

const datasets = ref<Dataset[]>([])
const err = ref<string | null>(null)

async function load() {
  err.value = null
  try {
    datasets.value = await projectsApi.listDatasets(projectId.value)
  } catch (e) {
    err.value = errorMessage(e)
  }
}

onMounted(load)

watch(projectId, () => {
  load()
})
</script>

<template>
  <div>
    <ProjectBreadcrumbs :project-id="projectId" :trail="[{ title: 'Datasets', disabled: true }]" />
    <h1 class="text-h5 mb-4">Saved datasets</h1>
    <v-alert v-if="err" type="error" class="mb-4">{{ err }}</v-alert>
    <v-list v-if="datasets.length" lines="three" border rounded>
      <v-list-item
        v-for="d in datasets"
        :key="d.id"
        :title="d.name"
        :subtitle="`#${d.id} · ${new Date(d.createdAt).toLocaleString()}`"
        @click="router.push({ name: 'dataset', params: { projectId: String(projectId), datasetId: String(d.id) } })"
      />
    </v-list>
    <v-alert v-else type="info" variant="tonal">No datasets yet — use “Save as dataset” from a collection.</v-alert>
  </div>
</template>
