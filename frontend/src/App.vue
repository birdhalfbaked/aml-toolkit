<script setup lang="ts">
import { RouterView, useRoute } from 'vue-router'
import { computed } from 'vue'

const route = useRoute()
const title = computed(() => {
  if (route.name === 'setup') return 'Setup'
  if (route.name === 'projects') return 'Audio tagger'
  if (route.name === 'project') return 'Project'
  if (route.name === 'labeling') return 'Labeling'
  if (route.name === 'datasets') return 'Datasets'
  if (route.name === 'dataset') return 'Dataset'
  return 'Audio tagger'
})
</script>

<template>
  <v-app>
    <v-app-bar color="surface" border="b">
      <v-app-bar-title>{{ title }}</v-app-bar-title>
      <v-spacer />
      <v-btn v-if="route.name !== 'projects' && route.name !== 'setup'" to="/" variant="text">All projects</v-btn>
    </v-app-bar>
    <v-main>
      <v-container fluid class="pa-4">
        <RouterView />
      </v-container>
    </v-main>
  </v-app>
</template>
