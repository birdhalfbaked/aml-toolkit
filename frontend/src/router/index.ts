import { createRouter, createWebHistory } from 'vue-router'
import ProjectsView from '@/views/ProjectsView.vue'
import ProjectHomeView from '@/views/ProjectHomeView.vue'
import LabelingView from '@/views/LabelingView.vue'
import DatasetsView from '@/views/DatasetsView.vue'
import DatasetDetailView from '@/views/DatasetDetailView.vue'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    { path: '/', name: 'projects', component: ProjectsView },
    { path: '/project/:projectId', name: 'project', component: ProjectHomeView },
    { path: '/project/:projectId/collection/:collectionId', name: 'labeling', component: LabelingView },
    { path: '/project/:projectId/datasets', name: 'datasets', component: DatasetsView },
    { path: '/project/:projectId/dataset/:datasetId', name: 'dataset', component: DatasetDetailView },
  ],
})

export default router
