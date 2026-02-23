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
        :indent="18"
        :root-indent="20"
        :expanded-keys="expandedKeys"
        :options="menuOptions"
        @update:value="handleMenuChange"
        @update:expanded-keys="expandedKeys = $event"
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
            <n-tag
              v-if="isDemoMode"
              size="small"
              type="warning"
              :bordered="false"
            >
              演示模式
            </n-tag>
            <n-tooltip v-if="runtimeInfo" trigger="hover" placement="bottom-end">
              <template #trigger>
                <n-tag size="small" type="info" :bordered="false" class="runtime-tag">
                  {{ runtimeLabel }}
                </n-tag>
              </template>
              <div class="runtime-popover">
                <div>版本: {{ runtimeInfo.version }}</div>
                <div>分支: {{ runtimeInfo.git_branch }}</div>
                <div>提交: {{ runtimeInfo.git_commit }}</div>
                <div>构建时间: {{ runtimeInfo.build_time }}</div>
                <div>进程: {{ runtimeInfo.pid }}</div>
                <div>静态源: {{ runtimeInfo.static_source || 'unknown' }}</div>
                <div>主资源: {{ runtimeAssetLabel }}</div>
                <div>启动时间: {{ formatRuntimeTime(runtimeInfo.started_at) }}</div>
              </div>
            </n-tooltip>
            <ProjectContextSelector />
            <ApprovalCenter />
            <n-badge :value="unreadCount" :max="99" :show="unreadCount > 0">
              <n-button quaternary size="small" @click="$router.push('/messages')">
                🔔
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
        :indent="18"
        :root-indent="20"
        :expanded-keys="expandedKeys"
        :options="menuOptions"
        @update:value="handleMobileMenuChange"
        @update:expanded-keys="expandedKeys = $event"
      />
    </n-drawer-content>
  </n-drawer>
</template>

<script setup lang="ts">
import { ref, onMounted, computed, h, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useIsMobile } from '@/utils/useIsMobile'
import ApprovalCenter from '@/components/ApprovalCenter.vue'
import ProjectContextSelector from '@/components/ProjectContextSelector.vue'
import type { RuntimeVersionInfo } from '@/api/types'
import { automationApi, runtimeApi } from '@/api'
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
  NBreadcrumbItem,
  NTag,
  NTooltip
} from 'naive-ui'
import type { MenuOption } from 'naive-ui'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()

const collapsed = ref(false)
const unreadCount = ref(0)
const runtimeInfo = ref<RuntimeVersionInfo | null>(null)
const user = computed(() => authStore.user)
const isDemoMode = computed(() => authStore.isDemoMode)
const showMobileMenu = ref(false)
const { isMobile } = useIsMobile()
const DEFAULT_EXPANDED_PARENT_KEYS = ['work', 'automation', 'ops', 'observe'] as const
const expandedKeys = ref<string[]>([...DEFAULT_EXPANDED_PARENT_KEYS])

const parentKeyByMenuKey: Record<string, string> = {
  'work-items': 'work',
  kanban: 'work',
  workflows: 'automation',
  schedules: 'automation',
  'ai-intelligence': 'automation',
  terminals: 'ops',
  servers: 'ops',
  messages: 'observe',
  audit: 'observe'
}

function icon(emoji: string) {
  return () => h('span', { style: 'font-size: 18px' }, emoji)
}

const leafMenu: Record<string, { label: string; path: string; icon: string }> = {
  dashboard: { label: '工作台', path: '/', icon: '🏠' },
  'work-items': { label: '工作清单', path: '/tasks', icon: '📋' },
  kanban: { label: '任务看板', path: '/kanban', icon: '📊' },
  workflows: { label: '工作流', path: '/workflows', icon: '🔁' },
  schedules: { label: '计划任务', path: '/schedules', icon: '⏱️' },
  'ai-intelligence': { label: 'AI 智能', path: '/ai-intelligence', icon: '🤖' },
  terminals: { label: '终端', path: '/terminals', icon: '🧪' },
  servers: { label: '服务器', path: '/servers', icon: '🖥️' },
  messages: { label: '消息中心', path: '/messages', icon: '🔔' },
  audit: { label: '审计', path: '/audit', icon: '🧾' },
  settings: { label: '系统设置', path: '/settings', icon: '⚙️' }
}

const baseMenuOptions: MenuOption[] = [
  {
    label: leafMenu.dashboard.label,
    key: 'dashboard',
    icon: icon(leafMenu.dashboard.icon)
  },
  {
    label: '工作',
    key: 'work',
    icon: icon('📦'),
    children: [
      {
        label: leafMenu['work-items'].label,
        key: 'work-items',
        icon: icon(leafMenu['work-items'].icon)
      },
      {
        label: leafMenu.kanban.label,
        key: 'kanban',
        icon: icon(leafMenu.kanban.icon)
      }
    ]
  },
  {
    label: '智能自动化',
    key: 'automation',
    icon: icon('🤖'),
    children: [
      {
        label: leafMenu.workflows.label,
        key: 'workflows',
        icon: icon(leafMenu.workflows.icon)
      },
      {
        label: leafMenu.schedules.label,
        key: 'schedules',
        icon: icon(leafMenu.schedules.icon)
      },
      {
        label: leafMenu['ai-intelligence'].label,
        key: 'ai-intelligence',
        icon: icon(leafMenu['ai-intelligence'].icon)
      }
    ]
  },
  {
    label: '执行与资源',
    key: 'ops',
    icon: icon('🧪'),
    children: [
      {
        label: leafMenu.terminals.label,
        key: 'terminals',
        icon: icon(leafMenu.terminals.icon)
      },
      {
        label: leafMenu.servers.label,
        key: 'servers',
        icon: icon(leafMenu.servers.icon)
      }
    ]
  },
  {
    label: '观察与审计',
    key: 'observe',
    icon: icon('🧾'),
    children: [
      {
        label: leafMenu.messages.label,
        key: 'messages',
        icon: icon(leafMenu.messages.icon)
      },
      {
        label: leafMenu.audit.label,
        key: 'audit',
        icon: icon(leafMenu.audit.icon)
      }
    ]
  },
  {
    label: leafMenu.settings.label,
    key: 'settings',
    icon: icon(leafMenu.settings.icon)
  }
]

const menuOptions = computed<MenuOption[]>(() => baseMenuOptions)
const runtimeLabel = computed(() => {
  const info = runtimeInfo.value
  if (!info) return ''
  const branch = info.git_branch || 'unknown'
  const commit = info.git_commit && info.git_commit !== 'unknown'
    ? info.git_commit.slice(0, 8)
    : 'unknown'
  return `${branch}@${commit}`
})
const runtimeAssetLabel = computed(() => {
  const assets = runtimeInfo.value?.static_index_assets || []
  return assets.length > 0 ? assets.join(', ') : '无'
})

const mobileNavItems: Array<{ key: string; label: string; icon: string }> = [
  { key: 'dashboard', label: '工作台', icon: '🏠' },
  { key: 'work-items', label: '工作', icon: '📋' },
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
  if (path.startsWith('/projects')) return 'work-items'
  if (path.startsWith('/tasks') || path.startsWith('/task/')) return 'work-items'
  if (path.startsWith('/workflows')) return 'workflows'
  if (path.startsWith('/schedules')) return 'schedules'
  if (path.startsWith('/messages')) return 'messages'
  if (path.startsWith('/audit') || path.startsWith('/logs')) return 'audit'
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
  if (path.startsWith('/tasks') || path.startsWith('/task/')) return '工作清单'
  if (path.startsWith('/workflows')) return '工作流'
  if (path.startsWith('/schedules')) return '计划任务'
  if (path.startsWith('/messages')) return '消息中心'
  if (path.startsWith('/audit') || path.startsWith('/logs')) return '审计'
  if (path.startsWith('/terminals')) return '终端管理'
  if (path.startsWith('/settings')) return '系统设置'
  return null
})

function handleMenuChange(key: string) {
  const target = leafMenu[key]
  if (!target) return
  router.push(target.path)
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

async function fetchRuntimeInfo() {
  try {
    const { data } = await runtimeApi.getVersion()
    runtimeInfo.value = data?.item || null
  } catch {
    runtimeInfo.value = null
  }
}

function formatRuntimeTime(raw: string) {
  if (!raw || raw === 'unknown') return raw || 'unknown'
  const date = new Date(raw)
  if (Number.isNaN(date.getTime())) return raw
  return date.toLocaleString()
}

onMounted(() => {
  authStore.fetchUser()
  fetchRuntimeInfo()
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

watch(activeMenu, (key) => {
  const parent = parentKeyByMenuKey[key]
  if (!parent) return
  if (!expandedKeys.value.includes(parent)) {
    expandedKeys.value = [...expandedKeys.value, parent]
  }
}, { immediate: true })

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

.runtime-tag {
  cursor: default;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace;
}

.runtime-popover {
  min-width: 260px;
  max-width: 440px;
  font-size: 12px;
  line-height: 1.55;
  word-break: break-all;
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

:deep(.n-submenu > .n-menu-item .n-menu-item-content-header) {
  font-weight: 600;
  opacity: 0.95;
}

:deep(.n-submenu-children .n-menu-item-content-header) {
  font-weight: 500;
  opacity: 0.9;
}
</style>
