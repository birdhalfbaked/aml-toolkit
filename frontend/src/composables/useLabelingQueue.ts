import { ref, watch, type Ref } from 'vue'
import * as audioApi from '@/api/audio'
import type { AudioFile, LabelingQueueItem } from '@/types'
import { errorMessage } from '@/utils/error'

export function useLabelingQueue(collectionId: Ref<number>) {
  const queue = ref<LabelingQueueItem[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function load() {
    loading.value = true
    error.value = null
    try {
      queue.value = (await audioApi.labelingQueue(collectionId.value)) ?? []
    } catch (e) {
      error.value = errorMessage(e)
    } finally {
      loading.value = false
    }
  }

  watch(collectionId, load, { immediate: true })

  function nextFileId(currentId: number | null, files: AudioFile[]): number | null {
    const need = new Set(queue.value.map((q) => q.audioFileId))
    const ordered = [...files].sort((a, b) => a.id - b.id)
    if (!ordered.length) return null
    if (currentId == null) {
      const first = ordered.find((f) => need.has(f.id))
      return first?.id ?? ordered[0].id
    }
    const curIdx = ordered.findIndex((f) => f.id === currentId)
    for (let i = curIdx + 1; i < ordered.length; i++) {
      if (need.has(ordered[i].id)) return ordered[i].id
    }
    const first = ordered.find((f) => need.has(f.id))
    if (first && first.id !== currentId) return first.id
    return null
  }

  return { queue, loading, error, load, nextFileId }
}
