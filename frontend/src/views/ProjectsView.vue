<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import * as projectsApi from '@/api/projects'
import type { Project } from '@/types'
import { errorMessage } from '@/utils/error'

const router = useRouter()
const projects = ref<Project[]>([])
const name = ref('')
const err = ref<string | null>(null)
const loading = ref(false)

async function load() {
  loading.value = true
  err.value = null
  try {
    projects.value = (await projectsApi.listProjects()) ?? []
  } catch (e) {
    err.value = errorMessage(e)
  } finally {
    loading.value = false
  }
}

async function create() {
  if (!name.value.trim()) return
  err.value = null
  try {
    const p = await projectsApi.createProject(name.value.trim())
    if (p == null || p.id == null) {
      throw new Error('Server returned no project (empty API body). Try rebuilding the desktop app.')
    }
    name.value = ''
    await load()
    router.push({ name: 'project', params: { projectId: String(p.id) } })
  } catch (e) {
    err.value = errorMessage(e)
  }
}

onMounted(load)
</script>

<template>
  <div>
    <h1 class="text-h5 mb-4">Projects</h1>
    <v-alert v-if="err" type="error" class="mb-4">{{ err }}</v-alert>
    <v-row class="mb-4" align="center">
      <v-col cols="12" md="6">
        <v-text-field v-model="name" label="New project name" density="comfortable" hide-details @keyup.enter="create" />
      </v-col>
      <v-col cols="auto">
        <v-btn color="primary" :loading="loading" @click="create">Create</v-btn>
      </v-col>
    </v-row>
    <v-list v-if="projects.length" lines="two" border rounded>
      <v-list-item
        v-for="p in projects"
        :key="p.id"
        :title="p.name"
        :subtitle="`Project #${p.id}`"
        @click="router.push({ name: 'project', params: { projectId: String(p.id) } })"
      />
    </v-list>
    <v-alert v-else-if="!loading" type="info" variant="tonal">No projects yet - create one above.</v-alert>
  </div>
</template>
