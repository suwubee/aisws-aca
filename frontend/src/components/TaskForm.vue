<template>
  <n-form :model="model" label-placement="left" label-width="70" size="small">
    <n-form-item label="标题" required>
      <n-input v-model:value="model.title" placeholder="任务标题" :disabled="disabled" />
    </n-form-item>
    <n-form-item label="描述">
      <n-input
        v-model:value="model.description"
        type="textarea"
        :rows="2"
        placeholder="任务描述（可选）"
        :disabled="disabled"
      />
    </n-form-item>
    <n-grid :cols="2" :x-gap="12">
      <n-gi>
        <n-form-item label="优先级">
          <n-select
            v-model:value="model.priority"
            :options="priorityOptions"
            :disabled="disabled"
          />
        </n-form-item>
      </n-gi>
      <n-gi>
        <n-form-item label="服务器">
          <n-select
            v-model:value="model.server_id"
            :options="serverStore.serverOptions"
            :loading="serverStore.loading"
            clearable
            placeholder="本地"
            :disabled="disabled"
          />
        </n-form-item>
      </n-gi>
    </n-grid>

    <n-divider style="margin: 8px 0">自动化配置</n-divider>

    <n-grid :cols="2" :x-gap="12">
      <n-gi>
        <n-form-item label="工作目录">
          <n-input v-model:value="model.work_dir" placeholder="/path/to/project" :disabled="disabled" />
        </n-form-item>
      </n-gi>
      <n-gi>
        <n-form-item label="CLI">
          <n-select
            v-model:value="model.cli_type"
            :options="cliOptions"
            :disabled="disabled"
          />
        </n-form-item>
      </n-gi>
    </n-grid>
    <n-form-item label="初始提示">
      <n-input
        v-model:value="model.initial_prompt"
        type="textarea"
        :rows="2"
        placeholder="启动后自动输入的提示内容"
        :disabled="disabled"
      />
    </n-form-item>
    <n-form-item label="选项">
      <n-space size="small">
        <n-checkbox v-model:checked="model.auto_create_dir" :disabled="disabled">自动创建目录</n-checkbox>
        <n-checkbox v-model:checked="model.auto_start" :disabled="disabled">自动启动</n-checkbox>
        <n-checkbox v-model:checked="model.return_to_workbench" :disabled="disabled">返回工作台</n-checkbox>
      </n-space>
    </n-form-item>
  </n-form>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useServerStore } from '@/stores/server'

export interface TaskFormModel {
  title: string
  description: string
  priority: number
  server_id: string | null
  work_dir: string
  cli_type: string
  initial_prompt: string
  auto_create_dir: boolean
  auto_start: boolean
  return_to_workbench: boolean
}

defineProps<{
  model: TaskFormModel
  disabled?: boolean
}>()

const serverStore = useServerStore()

const cliOptions = [
  { label: 'Claude Code', value: 'claude' },
  { label: 'Codex', value: 'codex' },
  { label: 'Gemini CLI', value: 'gemini' }
]

const priorityOptions = [
  { label: '低', value: 0 },
  { label: '中', value: 1 },
  { label: '高', value: 2 },
  { label: '紧急', value: 3 }
]

onMounted(() => {
  serverStore.fetchServers().catch(() => {})
})
</script>

