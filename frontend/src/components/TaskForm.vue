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
    <n-form-item label="备注">
      <n-input
        v-model:value="model.remark"
        type="textarea"
        :rows="2"
        placeholder="可用于临时提醒/记录关键结论（可选）"
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
        <n-form-item label="方式">
          <n-select
            v-model:value="model.automation_mode"
            :options="automationModeOptions"
            :disabled="disabled"
          />
        </n-form-item>
      </n-gi>
    </n-grid>

    <n-form-item v-if="model.automation_mode === 'script' || model.automation_mode === 'agent'" label="目标服务器">
      <n-select
        v-model:value="model.target_server_ids"
        :options="serverStore.serverOptions"
        :loading="serverStore.loading"
        clearable
        multiple
        placeholder="请选择目标服务器（本地也需要添加为服务器记录）"
        :disabled="disabled"
      />
    </n-form-item>
    <n-form-item v-else label="服务器">
      <n-select
        v-model:value="model.server_id"
        :options="serverStore.serverOptions"
        :loading="serverStore.loading"
        clearable
        placeholder="请选择服务器（本地也需要添加为服务器记录）"
        :disabled="disabled"
      />
    </n-form-item>

    <n-form-item label="项目">
      <n-select
        v-model:value="model.project_id"
        :options="projectStore.projectOptions"
        :loading="projectStore.loadingProjects || projectStore.loadingGroups"
        clearable
        filterable
        placeholder="未关联（可选）"
        :disabled="disabled"
      />
    </n-form-item>

    <n-divider v-if="model.automation_mode !== 'none'" style="margin: 8px 0">自动化配置</n-divider>

    <template v-if="model.automation_mode === 'cli'">
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
          <n-checkbox v-model:checked="model.ai_managed" :disabled="disabled">AI全程托管</n-checkbox>
          <n-checkbox v-model:checked="model.return_to_workbench" :disabled="disabled">返回工作台</n-checkbox>
        </n-space>
      </n-form-item>
    </template>

    <template v-else-if="model.automation_mode === 'agent'">
      <n-form-item label="工作目录">
        <n-input v-model:value="model.work_dir" placeholder="~/（可选，默认空）" :disabled="disabled" />
      </n-form-item>
      <n-form-item label="任务目标">
        <n-input
          v-model:value="model.initial_prompt"
          type="textarea"
          :rows="3"
          placeholder="描述要达成的目标（AI将根据命令返回动态决定下一步）"
          :disabled="disabled"
        />
      </n-form-item>
      <n-form-item label="选项">
        <n-space size="small">
          <n-checkbox v-model:checked="model.auto_start" :disabled="disabled">自动启动</n-checkbox>
          <n-checkbox v-model:checked="model.return_to_workbench" :disabled="disabled">返回工作台</n-checkbox>
        </n-space>
      </n-form-item>

      <n-divider style="margin: 8px 0">AI托管配置</n-divider>
      <n-form-item label="托管提示">
        <n-input
          v-model:value="model.ai_prompt"
          type="textarea"
          :rows="2"
          placeholder="可选：约束/策略/注意事项（例如先巡检、再批量执行；危险操作先询问）"
          :disabled="disabled"
        />
      </n-form-item>
      <n-form-item label="结束条件">
        <n-input
          v-model:value="model.ai_end_condition"
          placeholder="可选：任务什么时候算完成"
          :disabled="disabled"
        />
      </n-form-item>
      <n-form-item label="错误处理">
        <n-select
          v-model:value="model.ai_error_handling"
          :options="errorHandlingOptions"
          :disabled="disabled"
        />
      </n-form-item>
    </template>

    <template v-else-if="model.automation_mode === 'script'">
      <n-form-item label="工作目录">
        <n-input v-model:value="model.work_dir" placeholder="~/（可选，默认自动分配）" :disabled="disabled" />
      </n-form-item>
      <n-form-item label="脚本">
        <n-input
          v-model:value="model.script"
          type="textarea"
          :rows="8"
          placeholder="支持多行脚本，适合 runbook/批量运维场景"
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
    </template>

    <template v-else>
      <n-form-item label="选项">
        <n-space size="small">
          <n-checkbox v-model:checked="model.return_to_workbench" :disabled="disabled">返回工作台</n-checkbox>
        </n-space>
      </n-form-item>
    </template>

    <!-- AI托管配置 -->
    <template v-if="model.automation_mode === 'cli' && model.ai_managed">
      <n-divider style="margin: 8px 0">AI托管配置</n-divider>
      <n-form-item label="托管提示">
        <n-input
          v-model:value="model.ai_prompt"
          type="textarea"
          :rows="2"
          placeholder="AI在什么情况执行什么动作（可选）"
          :disabled="disabled"
        />
      </n-form-item>
      <n-form-item label="结束条件">
        <n-input
          v-model:value="model.ai_end_condition"
          placeholder="任务什么时候算完成（可选）"
          :disabled="disabled"
        />
      </n-form-item>
      <n-form-item label="错误处理">
        <n-select
          v-model:value="model.ai_error_handling"
          :options="errorHandlingOptions"
          :disabled="disabled"
        />
      </n-form-item>
    </template>
  </n-form>
</template>

<script setup lang="ts">
import { onMounted, watch } from 'vue'
import { useServerStore } from '@/stores/server'
import { useProjectStore } from '@/stores/project'

export interface TaskFormModel {
  title: string
  description: string
  remark: string
  priority: number
  server_id: string | null
  project_id: string | null
  automation_mode: string
  target_server_ids: string[]
  script: string
  work_dir: string
  cli_type: string
  initial_prompt: string
  auto_create_dir: boolean
  auto_start: boolean
  return_to_workbench: boolean
  // AI托管配置
  ai_managed: boolean
  ai_prompt: string
  ai_end_condition: string
  ai_error_handling: string
}

const props = defineProps<{
  model: TaskFormModel
  disabled?: boolean
}>()

const model = props.model

const serverStore = useServerStore()
const projectStore = useProjectStore()

const automationModeOptions = [
  { label: '仅记录', value: 'none' },
  { label: 'AI CLI', value: 'cli' },
  { label: 'AI 托管(动态)', value: 'agent' },
  { label: '脚本 / Runbook', value: 'script' }
]

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

const errorHandlingOptions = [
  { label: '暂停等待', value: 'pause' },
  { label: '自动重试', value: 'retry' },
  { label: '标记失败', value: 'fail' }
]

onMounted(() => {
  serverStore.fetchServers({ force: true }).catch(() => {})
  projectStore.fetchAll().catch(() => {})
})

watch(
  () => String(model.automation_mode || '').trim().toLowerCase(),
  (mode) => {
    if (mode === 'script' || mode === 'agent') {
      if ((!model.target_server_ids || model.target_server_ids.length === 0) && model.server_id) {
        model.target_server_ids = [model.server_id]
      }
      return
    }

    if (mode === 'cli') {
      if (!model.server_id && Array.isArray(model.target_server_ids) && model.target_server_ids.length === 1) {
        model.server_id = model.target_server_ids[0] || null
      }
    }
  },
  { immediate: true }
)
</script>
