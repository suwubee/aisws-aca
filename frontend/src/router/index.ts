import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const routes = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/views/Login.vue')
  },
  {
    path: '/',
    name: 'Main',
    component: () => import('@/views/MainLayout.vue'),
    meta: { requiresAuth: true },
    children: [
      {
        path: '',
        name: 'Dashboard',
        component: () => import('@/views/Dashboard.vue')
      },
      {
        path: 'task/:id',
        name: 'TaskDetail',
        component: () => import('@/views/TaskDetail.vue')
      },
      {
        path: 'tasks',
        name: 'Tasks',
        component: () => import('@/views/Tasks.vue')
      },
      {
        path: 'kanban',
        name: 'Kanban',
        component: () => import('@/views/KanbanView.vue')
      },
      {
        path: 'ai-intelligence',
        name: 'AIIntelligence',
        component: () => import('@/views/AIIntelligence.vue')
      },
      {
        path: 'messages',
        name: 'Messages',
        component: () => import('@/views/Messages.vue')
      },
      {
        path: 'logs',
        name: 'LogManagement',
        component: () => import('@/views/LogManagement.vue')
      },
      {
        path: 'terminals',
        name: 'Terminals',
        component: () => import('@/views/Terminals.vue')
      },
      {
        path: 'servers',
        name: 'Servers',
        component: () => import('@/views/Servers.vue')
      },
      {
        path: 'projects',
        name: 'Projects',
        component: () => import('@/views/Projects.vue')
      },
      {
        path: 'workflows',
        name: 'Workflows',
        component: () => import('@/views/Workflows.vue')
      },
      {
        path: 'schedules',
        name: 'Schedules',
        component: () => import('@/views/Schedules.vue')
      },
      {
        path: 'settings',
        name: 'Settings',
        component: () => import('@/views/Settings.vue')
      }
    ]
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

router.beforeEach((to, from, next) => {
  const authStore = useAuthStore()
  void from

  if (to.meta.requiresAuth && !authStore.isAuthenticated) {
    next('/login')
  } else if (to.path === '/login' && authStore.isAuthenticated) {
    next('/')
  } else {
    next()
  }
})

export default router
