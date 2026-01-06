<template>
  <div class="task-comments">
    <div class="comment-form">
      <div class="comment-form-header">
        <n-input
          v-model:value="author"
          size="small"
          placeholder="作者（可选）"
          class="author-input"
          :disabled="authorLocked"
        />
        <div class="comment-form-actions">
          <n-button size="small" :loading="manualRefreshing" @click="fetchComments({ indicator: 'manual' })">
            刷新
          </n-button>
          <n-button
            size="small"
            type="primary"
            :loading="submitting"
            :disabled="!newContent.trim()"
            @click="handleAdd"
          >
            发布
          </n-button>
        </div>
      </div>

      <n-input
        v-model:value="newContent"
        type="textarea"
        placeholder="写下评论..."
        :autosize="{ minRows: 2, maxRows: 6 }"
        @keydown.ctrl.enter.prevent="handleAdd"
      />
    </div>

    <div class="comment-list-header">
      <div class="comment-list-title">
        <span>评论</span>
        <span class="comment-count">({{ comments.length }})</span>
      </div>
    </div>

    <n-spin :show="loading">
      <n-empty v-if="!loading && comments.length === 0" description="暂无评论" />

      <n-list v-else bordered>
        <n-list-item v-for="comment in comments" :key="comment.id" class="comment-list-item">
          <div class="comment-item">
            <div class="comment-meta">
              <div class="comment-author">{{ comment.author || '匿名' }}</div>
              <div class="comment-time">{{ formatTime(comment.created_at) }}</div>
            </div>

            <div v-if="editingId === comment.id" class="comment-edit">
              <n-input
                v-model:value="editingContent"
                type="textarea"
                :autosize="{ minRows: 2, maxRows: 6 }"
              />
              <div class="comment-actions">
                <n-button
                  size="tiny"
                  type="primary"
                  :loading="savingId === comment.id"
                  :disabled="!editingContent.trim()"
                  @click="handleSave(comment.id)"
                >
                  保存
                </n-button>
                <n-button size="tiny" @click="cancelEdit">取消</n-button>
              </div>
            </div>

            <div v-else class="comment-body">
              <div class="comment-content">{{ comment.content }}</div>
              <div class="comment-actions">
                <n-button size="tiny" quaternary @click="startEdit(comment)">
                  编辑
                </n-button>
                <n-popconfirm
                  positive-text="删除"
                  negative-text="取消"
                  :disabled="deletingId === comment.id"
                  @positive-click="() => handleDelete(comment.id)"
                >
                  <template #trigger>
                    <n-button
                      size="tiny"
                      quaternary
                      type="error"
                      :loading="deletingId === comment.id"
                    >
                      删除
                    </n-button>
                  </template>
                  确定删除此评论？
                </n-popconfirm>
              </div>
            </div>
          </div>
        </n-list-item>
      </n-list>
    </n-spin>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useMessage } from 'naive-ui'
import { commentApi, type TaskComment } from '@/api'
import { useAuthStore } from '@/stores/auth'

const props = defineProps<{
  taskId: string
}>()

const message = useMessage()
const authStore = useAuthStore()

const comments = ref<TaskComment[]>([])
const loading = ref(false)
const manualRefreshing = ref(false)

const newContent = ref('')
const submitting = ref(false)

const author = ref('')
const authorLocked = computed(() => !!authStore.user?.username)

watch(
  () => authStore.user?.username,
  (username) => {
    if (username) author.value = username
  },
  { immediate: true }
)

const editingId = ref<string | null>(null)
const editingContent = ref('')
const savingId = ref<string | null>(null)
const deletingId = ref<string | null>(null)

let fetchSeq = 0
async function fetchComments(options: { silent?: boolean; indicator?: 'loading' | 'manual' | 'none' } = {}) {
  if (!props.taskId) return

  const silent = options.silent ?? false
  const indicator = options.indicator ?? 'loading'

  const seq = ++fetchSeq
  const flag = indicator === 'loading'
    ? loading
    : indicator === 'manual'
        ? manualRefreshing
        : null
  if (flag) flag.value = true

  try {
    const { data } = await commentApi.listByTask(props.taskId)
    if (seq !== fetchSeq) return

    const items = Array.isArray(data?.items) ? (data.items as TaskComment[]) : []
    comments.value = items

    if (editingId.value && !items.some(c => c.id === editingId.value)) {
      cancelEdit()
    }
  } catch (error: any) {
    if (!silent) {
      message.error(error.response?.data?.error || '加载评论失败')
    }
  } finally {
    if (flag) flag.value = false
  }
}

function startEdit(comment: TaskComment) {
  editingId.value = comment.id
  editingContent.value = comment.content || ''
}

function cancelEdit() {
  editingId.value = null
  editingContent.value = ''
}

function formatTime(time?: string) {
  if (!time) return '-'
  const date = new Date(time)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleString('zh-CN')
}

async function handleAdd() {
  const content = newContent.value.trim()
  if (!content) {
    message.warning('请输入评论内容')
    return
  }
  if (submitting.value) return

  submitting.value = true
  try {
    const effectiveAuthor = (authStore.user?.username || author.value.trim() || '匿名').trim()
    await commentApi.createForTask(props.taskId, { content, author: effectiveAuthor })
    newContent.value = ''
    message.success('评论已添加')
    await fetchComments({ silent: true, indicator: 'none' })
  } catch (error: any) {
    message.error(error.response?.data?.error || '添加评论失败')
  } finally {
    submitting.value = false
  }
}

async function handleSave(commentId: string) {
  const content = editingContent.value.trim()
  if (!content) {
    message.warning('请输入评论内容')
    return
  }
  if (savingId.value) return

  savingId.value = commentId
  try {
    await commentApi.update(commentId, { content })
    message.success('评论已更新')
    cancelEdit()
    await fetchComments({ silent: true, indicator: 'none' })
  } catch (error: any) {
    message.error(error.response?.data?.error || '更新评论失败')
  } finally {
    savingId.value = null
  }
}

async function handleDelete(commentId: string) {
  if (deletingId.value) return

  deletingId.value = commentId
  try {
    await commentApi.delete(commentId)
    message.success('评论已删除')
    await fetchComments({ silent: true, indicator: 'none' })
  } catch (error: any) {
    message.error(error.response?.data?.error || '删除评论失败')
  } finally {
    deletingId.value = null
  }
}

watch(
  () => props.taskId,
  () => {
    comments.value = []
    cancelEdit()
    newContent.value = ''
    fetchComments({ indicator: 'loading' })
  },
  { immediate: true }
)

let refreshTimer: number | null = null
onMounted(() => {
  refreshTimer = window.setInterval(() => fetchComments({ silent: true, indicator: 'none' }), 5000)
})

onUnmounted(() => {
  if (refreshTimer) clearInterval(refreshTimer)
})
</script>

<style scoped>
.task-comments {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.comment-form {
  border: 1px solid var(--border-color);
  border-radius: 6px;
  padding: 12px;
  background: rgba(255, 255, 255, 0.03);
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.comment-form-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.author-input {
  max-width: 220px;
}

.comment-form-actions {
  display: flex;
  gap: 8px;
}

.comment-list-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.comment-list-title {
  font-weight: 600;
  display: flex;
  gap: 6px;
  align-items: center;
}

.comment-count {
  color: #888;
  font-weight: 400;
}

.comment-item {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.comment-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  color: #888;
  font-size: 12px;
}

.comment-author {
  font-weight: 600;
  color: #d0d0d0;
}

.comment-content {
  white-space: pre-wrap;
  word-break: break-word;
}

.comment-actions {
  display: flex;
  gap: 6px;
  justify-content: flex-end;
}
</style>
