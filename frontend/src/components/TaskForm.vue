<template>
  <n-form :model="model">
    <n-form-item label="标题" required>
      <n-input v-model:value="model.title" placeholder="任务标题" :disabled="disabled" />
    </n-form-item>
    <n-form-item label="描述">
      <n-input
        v-model:value="model.description"
        type="textarea"
        placeholder="任务描述"
        :disabled="disabled"
      />
    </n-form-item>
    <n-form-item label="优先级">
      <n-radio-group v-model:value="model.priority" :disabled="disabled">
        <n-radio :value="0">低</n-radio>
        <n-radio :value="1">中</n-radio>
        <n-radio :value="2">高</n-radio>
        <n-radio :value="3">紧急</n-radio>
      </n-radio-group>
    </n-form-item>

    <n-form-item label="服务器">
      <n-select
        v-model:value="model.server_id"
        :options="serverStore.serverOptions"
        :loading="serverStore.loading"
        clearable
        placeholder="选择服务器（可选）"
        :disabled="disabled"
      />
    </n-form-item>

    <n-divider>自动化配置（可选）</n-divider>

    <n-form-item label="工作目录">
      <n-input v-model:value="model.work_dir" placeholder="/path/to/project" :disabled="disabled" />
    </n-form-item>
    <n-form-item label="CLI 类型">
      <n-select
        v-model:value="model.cli_type"
        :options="cliOptions"
        placeholder="选择 CLI 工具"
        :disabled="disabled"
      />
    </n-form-item>
    <n-form-item label="初始提示">
      <n-input
        v-model:value="model.initial_prompt"
        type="textarea"
        :rows="3"
        placeholder="启动后自动输入的提示内容"
        :disabled="disabled"
      />
    </n-form-item>
    <n-form-item label="选项">
      <n-space>
        <n-checkbox v-model:checked="model.auto_create_dir" :disabled="disabled">自动创建目录</n-checkbox>
        <n-checkbox v-model:checked="model.auto_start" :disabled="disabled">创建后自动启动</n-checkbox>
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

onMounted(() => {
  serverStore.fetchServers().catch(() => {})
})
</script>

