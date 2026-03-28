import { createRouter, createWebHistory } from 'vue-router'
import { getBootstrapStatus, clearBootstrapCache } from '@/api/bootstrap'
import ProjectsView from '@/views/ProjectsView.vue'
import ProjectHomeView from '@/views/ProjectHomeView.vue'
import LabelingView from '@/views/LabelingView.vue'
import DatasetsView from '@/views/DatasetsView.vue'
import DatasetDetailView from '@/views/DatasetDetailView.vue'
import SetupView from '@/views/SetupView.vue'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    { path: '/setup', name: 'setup', component: SetupView },
    { path: '/', name: 'projects', component: ProjectsView },
    { path: '/project/:projectId', name: 'project', component: ProjectHomeView },
    { path: '/project/:projectId/collection/:collectionId', name: 'labeling', component: LabelingView },
    { path: '/project/:projectId/datasets', name: 'datasets', component: DatasetsView },
    { path: '/project/:projectId/dataset/:datasetId', name: 'dataset', component: DatasetDetailView },
  ],
})

router.beforeEach(async (to, from, next) => {
  try {
    const st = await getBootstrapStatus()
    if (st.needsOnboarding && to.name !== 'setup') {
      next({ name: 'setup', replace: true })
      return
    }
    if (!st.needsOnboarding && to.name === 'setup') {
      next({ name: 'projects', replace: true })
      return
    }
    if (from.name === 'setup' && to.name !== 'setup') {
      clearBootstrapCache()
    }
    next()
  } catch {
    next()
  }
})

export default router
