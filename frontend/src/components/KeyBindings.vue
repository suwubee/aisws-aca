<template>
  <n-card size="small" title="全局按键绑定">
    <n-space vertical :size="12">
      <n-text depth="3">
        Enter（CR）用于“确认/选择”，Newline（LF）用于“文本换行”，两者不要混用。
      </n-text>

      <n-space justify="space-between" align="center">
        <n-text depth="3">用于终端快捷键与自动化输入（AI 审核/托管）。</n-text>
        <n-button size="small" quaternary @click="refresh" :loading="loading">刷新</n-button>
      </n-space>

      <div v-if="items.length === 0" class="empty">
        <n-text depth="3">暂无数据</n-text>
      </div>

      <div v-else class="cards">
        <n-card
          v-for="item in items"
          :key="item.id"
          size="small"
          class="binding-card"
        >
          <template #header>
            <n-space justify="space-between" align="center" :size="8">
              <n-space align="center" :size="8">
                <n-text strong>{{ item.label }}</n-text>
                <n-tag size="small" :bordered="false">{{ item.id }}</n-tag>
              </n-space>
              <n-space :size="8">
                <n-button v-if="isAdmin" size="tiny" @click="openEdit(item)">编辑</n-button>
                <n-popconfirm
                  v-if="isAdmin"
                  @positive-click="() => void resetOne(item.id)"
                  positive-text="恢复"
                  negative-text="取消"
                >
                  <template #trigger>
                    <n-button size="tiny" quaternary>恢复默认</n-button>
                  </template>
                  <span>确定恢复该按键为默认值？</span>
                </n-popconfirm>
              </n-space>
            </n-space>
          </template>

          <n-space vertical :size="8">
            <n-text depth="3">{{ item.description || '—' }}</n-text>
            <n-space :size="8" align="center">
              <n-text depth="3">PTY:</n-text>
              <n-text code>{{ item.pty_input || '—' }}</n-text>
            </n-space>
            <n-space :size="8" align="center">
              <n-text depth="3">tmux:</n-text>
              <n-text code>{{ item.tmux_keys || '—' }}</n-text>
              <n-tag v-if="item.tmux_literal" size="small" type="info" :bordered="false">literal</n-tag>
            </n-space>
          </n-space>
        </n-card>
      </div>
    </n-space>
  </n-card>

  <n-modal
    v-model:show="showEdit"
    preset="dialog"
    title="编辑按键绑定"
    positive-text="保存"
    negative-text="取消"
    style="width: min(560px, 94vw)"
    :loading="saving"
    @positive-click="saveEdit"
    @negative-click="closeEdit"
  >
    <n-form label-placement="left" label-width="110">
      <n-form-item label="ID">
        <n-input v-model:value="editForm.id" disabled />
      </n-form-item>
      <n-form-item label="显示名称">
        <n-input v-model:value="editForm.label" />
      </n-form-item>
      <n-form-item label="描述">
        <n-input v-model:value="editForm.description" type="textarea" :autosize="{ minRows: 2, maxRows: 4 }" />
      </n-form-item>
      <n-form-item label="PTY 输入">
        <n-input v-model:value="editForm.pty_input" placeholder="例如 \\r / y\\r / \\x03" />
      </n-form-item>
      <n-form-item label="tmux keys">
        <n-input v-model:value="editForm.tmux_keys" placeholder="例如 C-m / Escape / C-c" />
      </n-form-item>
      <n-form-item label="tmux literal">
        <n-checkbox v-model:checked="editForm.tmux_literal">使用 -l（按字面量发送）</n-checkbox>
      </n-form-item>
    </n-form>
  </n-modal>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { NButton, NCard, NCheckbox, NForm, NFormItem, NInput, NModal, NPopconfirm, NSpace, NTag, NText, useMessage } from 'naive-ui'
import { useAuthStore } from '@/stores/auth'
import { useKeyBindingsStore, type KeyBinding } from '@/stores/keyBindings'

const authStore = useAuthStore()
const keyBindingsStore = useKeyBindingsStore()
const message = useMessage()

const isAdmin = computed(() => authStore.isAdmin)
const items = computed(() => keyBindingsStore.items)
const loading = computed(() => keyBindingsStore.loading)

const showEdit = ref(false)
const saving = ref(false)
const editForm = reactive({
  id: '',
  label: '',
  description: '',
  pty_input: '',
  tmux_keys: '',
  tmux_literal: false
})

function openEdit(item: KeyBinding) {
  editForm.id = item.id
  editForm.label = item.label || ''
  editForm.description = item.description || ''
  editForm.pty_input = item.pty_input || ''
  editForm.tmux_keys = item.tmux_keys || ''
  editForm.tmux_literal = !!item.tmux_literal
  showEdit.value = true
}

function closeEdit() {
  showEdit.value = false
}

async function saveEdit() {
  if (!isAdmin.value) return
  saving.value = true
  try {
    await keyBindingsStore.update(editForm.id, {
      label: editForm.label,
      description: editForm.description,
      pty_input: editForm.pty_input,
      tmux_keys: editForm.tmux_keys,
      tmux_literal: editForm.tmux_literal
    })
    message.success('已保存')
    showEdit.value = false
  } catch (e: any) {
    message.error(e?.response?.data?.error || '保存失败')
  } finally {
    saving.value = false
  }
}

async function resetOne(id: string) {
  if (!isAdmin.value) return
  try {
    await keyBindingsStore.reset(id)
    message.success('已恢复默认')
  } catch (e: any) {
    message.error(e?.response?.data?.error || '恢复默认失败')
  }
}

function refresh() {
  void keyBindingsStore.fetchAll()
}

onMounted(() => {
  void keyBindingsStore.fetchAll()
})
</script>

<style scoped>
.cards {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: 12px;
}

.binding-card :deep(.n-card-header) {
  padding: 10px 12px;
}

.binding-card :deep(.n-card__content) {
  padding: 10px 12px;
}

.empty {
  padding: 12px 0;
}
</style>

