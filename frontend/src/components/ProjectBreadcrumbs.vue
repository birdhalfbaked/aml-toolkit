<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { RouteLocationRaw } from 'vue-router'
import * as projectsApi from '@/api/projects'

export type ProjectBreadcrumbTrailItem = {
  title: string
  to?: RouteLocationRaw
  disabled?: boolean
}

const props = withDefaults(
  defineProps<{
    projectId: number
    /** When set and non-empty after trim, skips fetching the project name */
    projectName?: string | null
    /** Project overview: project crumb is not a link */
    projectLinkDisabled?: boolean
    trail?: ProjectBreadcrumbTrailItem[]
  }>(),
  {
    projectLinkDisabled: false,
    trail: () => [],
  },
)

const resolvedName = ref('')

watch(
  () => [props.projectId, props.projectName] as const,
  async () => {
    const n = props.projectName != null ? String(props.projectName).trim() : ''
    if (n) {
      resolvedName.value = n
      return
    }
    try {
      const p = await projectsApi.getProject(props.projectId)
      resolvedName.value = p?.name ?? ''
    } catch {
      resolvedName.value = ''
    }
  },
  { immediate: true },
)

const items = computed(() => {
  const name = resolvedName.value.trim() || `Project #${props.projectId}`
  const second = props.projectLinkDisabled
    ? { title: name, disabled: true as const }
    : {
        title: name,
        to: { name: 'project' as const, params: { projectId: String(props.projectId) } },
      }
  return [{ title: 'Projects', to: '/' }, second, ...props.trail]
})
</script>

<template>
  <v-breadcrumbs :items="items" class="pa-0 mb-2" />
</template>
