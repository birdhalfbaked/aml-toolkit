<script setup lang="ts">
import * as bootstrapApi from '@/api/bootstrap'
import { canUseDesktopNativeAudio, desktopOpenLibraryFolder } from '@/lib/desktopWails'
import { errorMessage } from '@/utils/error'
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'

const router = useRouter()
const loading = ref(true)
const saving = ref(false)
const err = ref<string | null>(null)
const path = ref('')
const st = ref<bootstrapApi.BootstrapStatus | null>(null)

onMounted(async () => {
  try {
    const s = await bootstrapApi.getBootstrapStatus(true)
    st.value = s
    path.value = s.libraryRoot || s.recommendedLibraryRoot
  } catch (e) {
    err.value = errorMessage(e)
  } finally {
    loading.value = false
  }
})

async function browse() {
  if (!canUseDesktopNativeAudio()) return
  const p = await desktopOpenLibraryFolder()
  if (p) path.value = p
}

async function save() {
  const root = path.value.trim()
  if (!root) {
    err.value = 'Choose a folder first.'
    return
  }
  saving.value = true
  err.value = null
  try {
    await bootstrapApi.completeBootstrap(root)
    await router.replace({ name: 'projects' })
  } catch (e) {
    err.value = errorMessage(e)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="setup mx-auto" style="max-width: 640px">
    <h1 class="text-h5 mb-2">Welcome</h1>
    <p class="text-body-2 text-medium-emphasis mb-6">
      The app keeps its database under your user profile. Choose a folder on disk where projects and imported audio will
      be stored (you can use a cloud-synced folder if you like).
    </p>

    <v-alert v-if="err" type="error" class="mb-4" closable @click:close="err = null">{{ err }}</v-alert>

    <v-skeleton-loader v-if="loading" type="article" />

    <template v-else-if="st">
      <v-sheet border rounded class="pa-4 mb-4">
        <div class="text-caption text-medium-emphasis mb-1">SQLite database</div>
        <div class="text-body-2 text-truncate" :title="st.databasePath">{{ st.databasePath }}</div>
        <div class="text-caption text-medium-emphasis mt-3 mb-1">App settings directory</div>
        <div class="text-body-2 text-truncate" :title="st.stateDir">{{ st.stateDir }}</div>
      </v-sheet>

      <v-text-field
        v-model="path"
        label="Library folder (projects & audio)"
        density="comfortable"
        class="mb-2"
        persistent-hint
        :hint="`Suggested: ${st.recommendedLibraryRoot}`"
      />
      <div class="d-flex flex-wrap ga-2 mb-6">
        <v-btn v-if="canUseDesktopNativeAudio()" color="secondary" variant="tonal" @click="browse">Browse…</v-btn>
        <v-btn color="primary" :loading="saving" :disabled="saving" @click="save">Continue</v-btn>
      </div>
    </template>
  </div>
</template>
