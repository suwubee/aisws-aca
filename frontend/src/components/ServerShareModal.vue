<template>
  <n-modal
    v-model:show="showModal"
    preset="dialog"
    title="服务器共享管理"
    style="width: min(600px, 94vw)"
    :mask-closable="!loading"
    :close-on-esc="!loading"
  >
    <n-spin :show="loading">
      <div class="share-content">
        <n-alert v-if="server" type="info" :bordered="false" style="margin-bottom: 12px">
          服务器: {{ server.name || server.host }}
        </n-alert>

        <n-form-item label="选择要共享的用户">
          <n-select
            v-model:value="selectedUserIds"
            multiple
            filterable
            :options="userOptions"
            placeholder="选择用户..."
            :loading="loadingUsers"
          />
        </n-form-item>

        <div v-if="currentShares.length > 0" class="current-shares">
          <n-text depth="3" style="font-size: 13px">当前已共享给:</n-text>
          <n-space style="margin-top: 8px" size="small">
            <n-tag
              v-for="user in currentShares"
              :key="user.id"
              closable
              @close="handleRemoveShare(user.id)"
            >
              {{ user.username }}
            </n-tag>
          </n-space>
        </div>
      </div>
    </n-spin>

    <template #action>
      <n-space justify="end">
        <n-button :disabled="loading" @click="handleClose">取消</n-button>
        <n-button type="primary" :loading="saving" @click="handleSave">保存</n-button>
      </n-space>
    </template>
  </n-modal>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useMessage } from 'naive-ui'
import type { SSHServer, SharedUser } from '@/api/server'
import { getServerShares, shareServer } from '@/api/server'
import { userApi } from '@/api'

const props = defineProps<{
  show: boolean
  server: SSHServer | null
}>()

const emit = defineEmits<{
  (e: 'update:show', value: boolean): void
  (e: 'saved'): void
}>()

const message = useMessage()
const showModal = computed({
  get: () => props.show,
  set: (v) => emit('update:show', v)
})

const loading = ref(false)
const loadingUsers = ref(false)
const saving = ref(false)
const selectedUserIds = ref<string[]>([])
const currentShares = ref<SharedUser[]>([])
const allUsers = ref<Array<{ id: string; username: string; email: string }>>([])

const userOptions = computed(() => {
  return allUsers.value.map(u => ({
    label: `${u.username} (${u.email})`,
    value: u.id
  }))
})

watch(() => props.show, async (show) => {
  if (show && props.server) {
    await loadData()
  }
})

async function loadData() {
  if (!props.server) return
  loading.value = true
  try {
    const [sharesRes, usersRes] = await Promise.all([
      getServerShares(props.server.id),
      userApi.list()
    ])
    currentShares.value = sharesRes.data.items || []
    allUsers.value = usersRes.data.items || []
    selectedUserIds.value = currentShares.value.map(s => s.id)
  } catch (e: any) {
    message.error(e.response?.data?.error || '加载数据失败')
  } finally {
    loading.value = false
  }
}

function handleRemoveShare(userId: string) {
  selectedUserIds.value = selectedUserIds.value.filter(id => id !== userId)
}

function handleClose() {
  showModal.value = false
}

async function handleSave() {
  if (!props.server) return
  saving.value = true
  try {
    await shareServer(props.server.id, selectedUserIds.value)
    message.success('共享设置已保存')
    emit('saved')
    handleClose()
  } catch (e: any) {
    message.error(e.response?.data?.error || '保存失败')
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.share-content {
  min-height: 120px;
}

.current-shares {
  margin-top: 16px;
  padding-top: 12px;
  border-top: 1px solid rgba(255, 255, 255, 0.1);
}
</style>
