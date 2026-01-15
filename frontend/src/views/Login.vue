<template>
  <div class="login-container">
    <n-card class="login-card" title="AI Coding Assistant">
      <n-form ref="formRef" :model="form" :rules="rules">
        <n-form-item path="username" label="用户名">
          <n-input v-model:value="form.username" placeholder="请输入用户名" />
        </n-form-item>
        <n-form-item path="password" label="密码">
          <n-input
            v-model:value="form.password"
            type="password"
            placeholder="请输入密码"
            @keyup.enter="handleLogin"
          />
        </n-form-item>
        <n-button
          type="primary"
          block
          :loading="loading"
          @click="handleLogin"
        >
          登录
        </n-button>
      </n-form>
      <template #footer>
        <n-space vertical align="center">
          <n-button text type="info" @click="fillDemo">
            点击填充演示账号 (demo / demo123)
          </n-button>
        </n-space>
      </template>
    </n-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { useMessage } from 'naive-ui'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const message = useMessage()
const authStore = useAuthStore()

const loading = ref(false)
const form = reactive({
  username: '',
  password: ''
})

const rules = {
  username: { required: true, message: '请输入用户名' },
  password: { required: true, message: '请输入密码' }
}

function fillDemo() {
  form.username = 'demo'
  form.password = 'demo123'
}

async function handleLogin() {
  if (!form.username || !form.password) {
    message.warning('请输入用户名和密码')
    return
  }

  loading.value = true
  try {
    await authStore.login(form.username, form.password)
    message.success('登录成功')
    router.push('/')
  } catch (error: any) {
    message.error(error.response?.data?.error || '登录失败')
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-container {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #1a1a2e 0%, #16213e 100%);
}

.login-card {
  width: 400px;
  background: rgba(255, 255, 255, 0.05);
  backdrop-filter: blur(10px);
}
</style>
