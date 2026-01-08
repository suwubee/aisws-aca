<template>
  <n-layout class="main-layout" has-sider>
    <!-- 侧边栏 -->
    <n-layout-sider
      class="sider"
      :collapsed="collapsed"
      :collapsed-width="64"
      :width="200"
      collapse-mode="width"
      show-trigger="bar"
      bordered
      @update:collapsed="collapsed = $event"
    >
      <div class="logo" :class="{ collapsed }">
        <span class="logo-icon">🤖</span>
        <span v-if="!collapsed" class="logo-text">ACA</span>
      </div>
      <n-menu
        :value="activeMenu"
        :collapsed="collapsed"
        :collapsed-width="64"
        :collapsed-icon-size="20"
        :options="menuOptions"
        @update:value="handleMenuChange"
      />
    </n-layout-sider>

    <!-- 主内容区 -->
    <n-layout>
      <n-layout-header class="header">
        <div class="header-left">
          <n-breadcrumb>
            <n-breadcrumb-item>
              <router-link to="/">首页</router-link>
            </n-breadcrumb-item>
            <n-breadcrumb-item v-if="currentPageName">
              {{ currentPageName }}
            </n-breadcrumb-item>
          </n-breadcrumb>
        </div>
        <div class="header-right">
          <n-space align="center">
            <n-badge :value="unreadCount" :max="99" :show="unreadCount > 0">
              <n-button quaternary size="small" @click="$router.push('/logs')">
                📋
              </n-button>
            </n-badge>
            <n-dropdown :options="userOptions" @select="handleUserAction">
              <n-button quaternary size="small">
                <n-avatar :size="24" round>{{ user?.username?.[0]?.toUpperCase() || 'U' }}</n-avatar>
                <span style="margin-left: 8px;">{{ user?.username }}</span>
              </n-button>
            </n-dropdown>
          </n-space>
        </div>
      </n-layout-header>
      <n-layout-content class="content">
        <router-view />
      </n-layout-content>
    </n-layout>
  </n-layout>
</template>

<script setup lang="ts">
import { ref, onMounted, computed, h, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import {
  NLayout,
  NLayoutSider,
  NLayoutHeader,
  NLayoutContent,
  NMenu,
  NAvatar,
  NButton,
  NSpace,
  NDropdown,
  NBadge,
  NBreadcrumb,
  NBreadcrumbItem
} from 'naive-ui'
import type { MenuOption } from 'naive-ui'
import { automationApi } from '@/api'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()

const collapsed = ref(false)
const unreadCount = ref(0)
const user = computed(() => authStore.user)

const menuOptions: MenuOption[] = [
  {
    label: '工作台',
    key: 'dashboard',
    icon: () => h('span', { style: 'font-size: 18px' }, '🏠')
  },
  {
    label: '任务看板',
    key: 'kanban',
    icon: () => h('span', { style: 'font-size: 18px' }, '📊')
  },
  {
    label: '任务管理',
    key: 'tasks',
    icon: () => h('span', { style: 'font-size: 18px' }, '📋')
  },
  {
    label: 'AI 智能',
    key: 'ai-intelligence',
    icon: () => h('span', { style: 'font-size: 18px' }, '🤖')
  },
  {
    label: '服务器管理',
    key: 'servers',
    icon: () => h('span', { style: 'font-size: 18px' }, '🖥️')
  },
  {
    label: '终端管理',
    key: 'terminals',
    icon: () => h('span', { style: 'font-size: 18px' }, '🧪')
  },
  {
    label: '工作流',
    key: 'workflows',
    icon: () => h('span', { style: 'font-size: 18px' }, '🔁')
  },
  {
    label: '日志管理',
    key: 'logs',
    icon: () => h('span', { style: 'font-size: 18px' }, '📝')
  },
  {
    label: '系统设置',
    key: 'settings',
    icon: () => h('span', { style: 'font-size: 18px' }, '⚙️')
  }
]

const userOptions = [
  { label: '个人信息', key: 'profile' },
  { type: 'divider', key: 'd1' },
  { label: '退出登录', key: 'logout' }
]

const activeMenu = computed(() => {
  const path = route.path
  if (path === '/') return 'dashboard'
  if (path.startsWith('/kanban')) return 'kanban'
  if (path.startsWith('/ai-intelligence')) return 'ai-intelligence'
  if (path.startsWith('/servers')) return 'servers'
  if (path.startsWith('/tasks') || path.startsWith('/task/')) return 'tasks'
  if (path.startsWith('/workflows')) return 'workflows'
  if (path.startsWith('/logs')) return 'logs'
  if (path.startsWith('/terminals')) return 'terminals'
  if (path.startsWith('/settings')) return 'settings'
  return 'dashboard'
})

const currentPageName = computed(() => {
  const path = route.path
  if (path === '/') return null
  if (path.startsWith('/kanban')) return '任务看板'
  if (path.startsWith('/ai-intelligence')) return 'AI 智能'
  if (path.startsWith('/servers')) return '服务器管理'
  if (path.startsWith('/tasks') || path.startsWith('/task/')) return '任务管理'
  if (path.startsWith('/workflows')) return '工作流'
  if (path.startsWith('/logs')) return '日志管理'
  if (path.startsWith('/terminals')) return '终端管理'
  if (path.startsWith('/settings')) return '系统设置'
  return null
})

function handleMenuChange(key: string) {
  switch (key) {
    case 'dashboard':
      router.push('/')
      break
    case 'kanban':
      router.push('/kanban')
      break
    case 'ai-intelligence':
      router.push('/ai-intelligence')
      break
    case 'servers':
      router.push('/servers')
      break
    case 'tasks':
      router.push('/tasks')
      break
    case 'workflows':
      router.push('/workflows')
      break
    case 'logs':
      router.push('/logs')
      break
    case 'terminals':
      router.push('/terminals')
      break
    case 'settings':
      router.push('/settings')
      break
  }
}

function handleUserAction(key: string) {
  if (key === 'logout') {
    handleLogout()
  }
}

async function handleLogout() {
  await authStore.logout()
  router.replace('/login')
}

async function fetchUnreadCount() {
  try {
    const { data } = await automationApi.getUnreadCount()
    unreadCount.value = data.count || 0
  } catch (e) {
    // ignore
  }
}

onMounted(() => {
  authStore.fetchUser()
  fetchUnreadCount()
  // 定期检查未读消息
  setInterval(fetchUnreadCount, 30000)
})
</script>

<style scoped>
.main-layout {
  min-height: 100vh;
}

.sider {
  background: #1a1f2e;
}

.logo {
  height: 56px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  border-bottom: 1px solid #334155;
  transition: all 0.3s;
}

.logo.collapsed {
  padding: 0;
}

.logo-icon {
  font-size: 24px;
}

.logo-text {
  font-size: 18px;
  font-weight: 700;
  color: #18a058;
}

.header {
  height: 56px;
  padding: 0 20px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: rgba(22, 33, 62, 0.95);
  border-bottom: 1px solid #334155;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 16px;
}

.header-right {
  display: flex;
  align-items: center;
}

.content {
  background: #0f1419;
  min-height: calc(100vh - 56px);
}

:deep(.n-menu) {
  background: transparent;
}

:deep(.n-menu-item-content) {
  padding-left: 20px !important;
}
</style>
