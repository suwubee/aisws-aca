<template>
  <n-layout class="main-layout" :class="{ 'main-layout--mobile': isMobile }" :has-sider="!isMobile">
    <!-- 侧边栏 -->
    <n-layout-sider
      v-if="!isMobile"
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
          <n-button
            v-if="isMobile"
            quaternary
            size="small"
            class="mobile-menu-btn"
            @click="showMobileMenu = true"
          >
            ☰
          </n-button>
          <div v-if="isMobile" class="mobile-title">
            {{ currentPageName || '首页' }}
          </div>
          <n-breadcrumb v-else>
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
              <n-button quaternary size="small" class="user-btn">
                <n-avatar :size="24" round>{{ user?.username?.[0]?.toUpperCase() || 'U' }}</n-avatar>
                <span v-if="!isMobile" class="username">{{ user?.username }}</span>
              </n-button>
            </n-dropdown>
          </n-space>
        </div>
      </n-layout-header>
      <n-layout-content class="content">
        <router-view />
      </n-layout-content>

      <div v-if="isMobile" class="mobile-bottom-nav">
        <button
          v-for="item in mobileNavItems"
          :key="item.key"
          class="mobile-bottom-nav__item"
          :class="{ active: activeMenu === item.key }"
          type="button"
          @click="handleMenuChange(item.key)"
        >
          <span class="mobile-bottom-nav__icon">{{ item.icon }}</span>
          <span class="mobile-bottom-nav__label">{{ item.label }}</span>
        </button>
      </div>
    </n-layout>
  </n-layout>

  <!-- Mobile Menu Drawer -->
  <n-drawer v-model:show="showMobileMenu" placement="left" :width="260">
    <n-drawer-content :native-scrollbar="false">
      <div class="logo">
        <span class="logo-icon">🤖</span>
        <span class="logo-text">ACA</span>
      </div>
      <n-menu
        :value="activeMenu"
        :options="menuOptions"
        @update:value="handleMobileMenuChange"
      />
    </n-drawer-content>
  </n-drawer>
</template>

<script setup lang="ts">
import { ref, onMounted, computed, h, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useIsMobile } from '@/utils/useIsMobile'
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
  NDrawer,
  NDrawerContent,
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
const showMobileMenu = ref(false)
const { isMobile } = useIsMobile()

const menuOptions: MenuOption[] = [
  {
    label: '工作台',
    key: 'dashboard',
    icon: () => h('span', { style: 'font-size: 18px' }, '🏠')
  },
  {
    label: '项目管理',
    key: 'projects',
    icon: () => h('span', { style: 'font-size: 18px' }, '📦')
  },
  {
    label: '服务器管理',
    key: 'servers',
    icon: () => h('span', { style: 'font-size: 18px' }, '🖥️')
  },
  {
    label: '任务管理',
    key: 'tasks',
    icon: () => h('span', { style: 'font-size: 18px' }, '📋')
  },
  {
    label: '任务看板',
    key: 'kanban',
    icon: () => h('span', { style: 'font-size: 18px' }, '📊')
  },
  {
    label: '工作流',
    key: 'workflows',
    icon: () => h('span', { style: 'font-size: 18px' }, '🔁')
  },
  {
    label: '终端管理',
    key: 'terminals',
    icon: () => h('span', { style: 'font-size: 18px' }, '🧪')
  },
  {
    label: 'AI 智能',
    key: 'ai-intelligence',
    icon: () => h('span', { style: 'font-size: 18px' }, '🤖')
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

const mobileNavItems: Array<{ key: string; label: string; icon: string }> = [
  { key: 'dashboard', label: '工作台', icon: '🏠' },
  { key: 'tasks', label: '任务', icon: '📋' },
  { key: 'kanban', label: '看板', icon: '📊' },
  { key: 'terminals', label: '终端', icon: '🧪' },
  { key: 'settings', label: '设置', icon: '⚙️' }
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
  if (path.startsWith('/projects')) return 'projects'
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
  if (path.startsWith('/projects')) return '项目管理'
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
    case 'projects':
      router.push('/projects')
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

function handleMobileMenuChange(key: string) {
  showMobileMenu.value = false
  handleMenuChange(key)
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

watch(
  () => route.path,
  () => {
    showMobileMenu.value = false
  }
)

watch(isMobile, (mobile) => {
  if (!mobile) showMobileMenu.value = false
})
</script>

<style scoped>
.main-layout {
  --app-header-height: 56px;
  --app-bottom-nav-height: 0px;
  min-height: 100vh;
}

.main-layout--mobile {
  --app-bottom-nav-height: 56px;
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
  height: var(--app-header-height);
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

.mobile-menu-btn {
  font-size: 18px;
  line-height: 1;
}

.mobile-title {
  font-size: 16px;
  font-weight: 600;
  color: #e2e8f0;
  max-width: 55vw;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.user-btn .username {
  margin-left: 8px;
}

.header-right {
  display: flex;
  align-items: center;
}

.content {
  background: #0f1419;
  min-height: calc(100vh - var(--app-header-height) - var(--app-bottom-nav-height));
  padding-bottom: calc(var(--app-bottom-nav-height) + env(safe-area-inset-bottom));
}

.mobile-bottom-nav {
  position: fixed;
  left: 0;
  right: 0;
  bottom: 0;
  height: calc(var(--app-bottom-nav-height) + env(safe-area-inset-bottom));
  padding-bottom: env(safe-area-inset-bottom);
  display: flex;
  align-items: stretch;
  background: rgba(22, 33, 62, 0.96);
  border-top: 1px solid #334155;
  z-index: 1000;
  backdrop-filter: blur(12px);
}

.mobile-bottom-nav__item {
  flex: 1;
  border: 0;
  background: transparent;
  color: rgba(226, 232, 240, 0.72);
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 2px;
  padding: 6px 0;
}

.mobile-bottom-nav__item.active {
  color: #e2e8f0;
}

.mobile-bottom-nav__icon {
  font-size: 18px;
  line-height: 1;
}

.mobile-bottom-nav__label {
  font-size: 11px;
  line-height: 1.1;
}

:deep(.n-menu) {
  background: transparent;
}

:deep(.n-menu-item-content) {
  padding-left: 20px !important;
}
</style>
