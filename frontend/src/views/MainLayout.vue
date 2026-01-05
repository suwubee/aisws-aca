<template>
  <n-layout class="main-layout">
    <n-layout-header class="header">
      <div class="header-left">
        <n-text strong style="font-size: 18px; color: #18a058;">
          AI Coding Assistant
        </n-text>
      </div>
      <div class="header-right">
        <n-space>
          <n-text>{{ user?.username }}</n-text>
          <n-button quaternary size="small" @click="handleLogout">
            退出
          </n-button>
        </n-space>
      </div>
    </n-layout-header>
    <n-layout-content class="content">
      <router-view />
    </n-layout-content>
  </n-layout>
</template>

<script setup lang="ts">
import { onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const authStore = useAuthStore()

const user = computed(() => authStore.user)

onMounted(() => {
  authStore.fetchUser()
})

function handleLogout() {
  authStore.logout()
  router.push('/login')
}
</script>

<style scoped>
.main-layout {
  min-height: 100vh;
}

.header {
  height: 56px;
  padding: 0 20px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: rgba(22, 33, 62, 0.95);
  border-bottom: 1px solid #334155;
  position: sticky;
  top: 0;
  z-index: 100;
}

.content {
  background: var(--bg-color);
}
</style>
