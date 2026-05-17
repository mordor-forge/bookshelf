import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import LibraryPage from './pages/LibraryPage.vue'
import ReaderPage from './pages/ReaderPage.vue'
import SettingsPage from './pages/SettingsPage.vue'

const routes: RouteRecordRaw[] = [
  { path: '/', name: 'library', component: LibraryPage },
  { path: '/read/:path(.*)', name: 'reader', component: ReaderPage, props: true },
  { path: '/settings', name: 'settings', component: SettingsPage },
]

export const router = createRouter({
  history: createWebHistory(),
  routes,
})
