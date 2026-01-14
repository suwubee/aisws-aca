<template>
  <n-modal v-model:show="showModal" preset="dialog" title="编辑任务" style="width: min(600px, 94vw)">
    <n-form :model="form">
      <n-form-item label="标题" required>
        <n-input v-model:value="form.title" placeholder="任务标题" />
      </n-form-item>
      <n-form-item label="描述">
        <n-input v-model:value="form.description" type="textarea" placeholder="任务描述" />
      </n-form-item>
      <n-form-item label="备注">
        <n-input v-model:value="form.remark" type="textarea" placeholder="可用于临时提醒/记录关键结论" />
      </n-form-item>
      <n-form-item label="优先级">
        <n-radio-group v-model:value="form.priority">
          <n-radio :value="0">低</n-radio>
          <n-radio :value="1">中</n-radio>
          <n-radio :value="2">高</n-radio>
          <n-radio :value="3">紧急</n-radio>
        </n-radio-group>
      </n-form-item>

      <n-form-item label="项目">
        <n-select
          v-model:value="form.project_id"
          :options="projectStore.projectOptions"
          :loading="projectStore.loadingProjects || projectStore.loadingGroups"
          clearable
          filterable
          placeholder="未关联（可选）"
        />
      </n-form-item>

      <n-divider>自动化配置</n-divider>

      <n-form-item label="方式">
        <n-select v-model:value="form.automation_mode" :options="modeOptions" />
      </n-form-item>

      <template v-if="form.automation_mode === 'cli'">
        <n-form-item label="服务器">
          <n-select
            v-model:value="form.server_id"
            :options="serverStore.serverOptions"
            :loading="serverStore.loading"
            clearable
            placeholder="请选择服务器（本地也需要添加为服务器记录）"
          />
        </n-form-item>
        <n-form-item label="工作目录">
          <n-input v-model:value="form.work_dir" placeholder="/path/to/project" />
        </n-form-item>
        <n-form-item label="CLI 类型">
          <n-select
            v-model:value="form.cli_type"
            :options="cliOptions"
            clearable
            placeholder="选择 CLI 工具（可选）"
          />
        </n-form-item>
        <n-form-item label="初始提示">
          <n-input
            v-model:value="form.initial_prompt"
            type="textarea"
            :rows="3"
            placeholder="启动后自动输入的提示内容（可选）"
          />
        </n-form-item>

        <n-divider>AI托管配置</n-divider>

        <n-form-item label="托管模式">
          <n-checkbox v-model:checked="form.ai_managed">AI全程托管</n-checkbox>
        </n-form-item>

        <template v-if="form.ai_managed">
          <n-form-item label="托管提示">
            <n-input
              v-model:value="form.ai_prompt"
              type="textarea"
              :rows="2"
              placeholder="AI在什么情况执行什么动作（可选）"
            />
          </n-form-item>
          <n-form-item label="结束条件">
            <n-input
              v-model:value="form.ai_end_condition"
              placeholder="任务什么时候算完成（可选）"
            />
          </n-form-item>
          <n-form-item label="错误处理">
            <n-select
              v-model:value="form.ai_error_handling"
              :options="errorHandlingOptions"
            />
          </n-form-item>
        </template>
      </template>

      <template v-else-if="form.automation_mode === 'agent'">
        <n-form-item label="目标服务器">
          <n-select
            v-model:value="form.target_server_ids"
            :options="serverStore.serverOptions"
            :loading="serverStore.loading"
            multiple
            clearable
            placeholder="请选择目标服务器（本地也需要添加为服务器记录）"
          />
        </n-form-item>
        <n-form-item label="工作目录">
          <n-input v-model:value="form.work_dir" placeholder="~/（可选，默认空）" />
        </n-form-item>
        <n-form-item label="任务目标">
          <n-input
            v-model:value="form.initial_prompt"
            type="textarea"
            :rows="3"
            placeholder="描述要达成的目标（AI将根据命令返回动态决定下一步）"
          />
        </n-form-item>

        <n-divider>AI托管配置</n-divider>
        <n-form-item label="托管提示">
          <n-input
            v-model:value="form.ai_prompt"
            type="textarea"
            :rows="2"
            placeholder="可选：约束/策略/注意事项"
          />
        </n-form-item>
        <n-form-item label="结束条件">
          <n-input
            v-model:value="form.ai_end_condition"
            placeholder="任务什么时候算完成（可选）"
          />
        </n-form-item>
        <n-form-item label="错误处理">
          <n-select
            v-model:value="form.ai_error_handling"
            :options="errorHandlingOptions"
          />
        </n-form-item>
      </template>

      <template v-else-if="form.automation_mode === 'script'">
        <n-form-item label="目标服务器">
          <n-select
            v-model:value="form.target_server_ids"
            :options="serverStore.serverOptions"
            :loading="serverStore.loading"
            multiple
            clearable
            placeholder="请选择目标服务器（本地也需要添加为服务器记录）"
          />
        </n-form-item>
        <n-form-item label="工作目录">
          <n-input v-model:value="form.work_dir" placeholder="~/（可选，默认自动分配）" />
        </n-form-item>
        <n-form-item label="脚本">
          <n-input
            v-model:value="form.script"
            type="textarea"
            :rows="8"
            placeholder="支持多行脚本，适合 runbook/批量运维场景"
          />
        </n-form-item>
      </template>
    </n-form>

    <template #action>
      <n-button :disabled="saving" @click="close">取消</n-button>
      <n-button type="primary" :loading="saving" @click="save">保存</n-button>
    </template>
  </n-modal>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useMessage } from 'naive-ui'
import { useTaskStore, type Task } from '@/stores/task'
import { useServerStore } from '@/stores/server'
import { useProjectStore } from '@/stores/project'

const props = defineProps<{
  show: boolean
  task: Task | null
}>()

const emit = defineEmits<{
  'update:show': [show: boolean]
  saved: [task: Task]
}>()

const message = useMessage()
const taskStore = useTaskStore()
const serverStore = useServerStore()
const projectStore = useProjectStore()

const showModal = computed({
  get: () => props.show,
  set: (value: boolean) => emit('update:show', value)
})

const saving = ref(false)
const form = reactive({
  title: '',
  description: '',
  remark: '',
  priority: 1,
  server_id: null as string | null,
  project_id: null as string | null,
  automation_mode: 'cli',
  target_server_ids: [] as string[],
  script: '',
  work_dir: '',
  cli_type: null as string | null,
  initial_prompt: '',
  // AI托管配置
  ai_managed: false,
  ai_prompt: '',
  ai_end_condition: '',
  ai_error_handling: 'pause'
})

const cliOptions = [
  { label: 'Claude Code', value: 'claude' },
  { label: 'Codex', value: 'codex' },
  { label: 'Gemini CLI', value: 'gemini' }
]

const modeOptions = [
  { label: '仅记录', value: 'none' },
  { label: 'AI CLI', value: 'cli' },
  { label: 'AI 托管(动态)', value: 'agent' },
  { label: '脚本 / Runbook', value: 'script' }
]

const errorHandlingOptions = [
  { label: '暂停等待', value: 'pause' },
  { label: '自动重试', value: 'retry' },
  { label: '标记失败', value: 'fail' }
]

function syncFormFromTask(task: Task | null) {
  const modeRaw = String(task?.automation_mode || '').trim().toLowerCase()
  form.automation_mode = modeRaw || 'cli'
  form.title = task?.title ?? ''
  form.description = task?.description ?? ''
  form.remark = task?.remark ?? ''
  const priority = typeof task?.priority === 'number' ? task.priority : 1
  form.priority = Math.min(3, Math.max(0, priority))
  form.server_id = task?.server_id ?? null
  form.project_id = task?.project_id ?? null
  form.target_server_ids = task?.target_server_ids ? [...task.target_server_ids] : []
  form.script = task?.script ?? ''
  form.work_dir = task?.work_dir ?? ''
  form.cli_type = task?.cli_type || null
  form.initial_prompt = task?.initial_prompt ?? ''
  // AI托管配置
  form.ai_managed = task?.ai_managed ?? false
  form.ai_prompt = task?.ai_prompt ?? ''
  form.ai_end_condition = task?.ai_end_condition ?? ''
  form.ai_error_handling = task?.ai_error_handling || 'pause'

  if ((form.automation_mode === 'script' || form.automation_mode === 'agent') && form.target_server_ids.length === 0 && form.server_id) {
    form.target_server_ids = [form.server_id]
  }
}

watch(
  () => [props.show, props.task] as const,
  ([show, task]) => {
    if (show) {
      projectStore.fetchAll().catch(() => {})
      serverStore.fetchServers().catch(() => {})
      syncFormFromTask(task)
    }
  },
  { immediate: true }
)

watch(
  () => String(form.automation_mode || '').trim().toLowerCase(),
  (mode) => {
    if (mode === 'script' || mode === 'agent') {
      if (form.target_server_ids.length === 0 && form.server_id) {
        form.target_server_ids = [form.server_id]
      }
      return
    }
    if (mode === 'cli') {
      if (!form.server_id && form.target_server_ids.length === 1) {
        form.server_id = form.target_server_ids[0]
      }
    }
  }
)

function close() {
  showModal.value = false
}

async function save() {
  if (saving.value) return
  if (!props.task) {
    message.error('未选择要编辑的任务')
    return
  }
  if (!form.title.trim()) {
    message.warning('请输入任务标题')
    return
  }

  saving.value = true
  try {
    const updated = await taskStore.updateTask(props.task.id, {
      title: form.title,
      description: form.description,
      remark: form.remark,
      priority: form.priority,
      project_id: form.project_id,
      automation_mode: form.automation_mode,
      server_id: form.automation_mode === 'cli' ? (form.server_id || '') : undefined,
      target_server_ids: (form.automation_mode === 'script' || form.automation_mode === 'agent') ? form.target_server_ids : undefined,
      script: form.automation_mode === 'script' ? form.script : undefined,
      work_dir: form.automation_mode === 'none' ? undefined : form.work_dir,
      cli_type: form.automation_mode === 'cli' ? (form.cli_type || '') : undefined,
      initial_prompt: (form.automation_mode === 'cli' || form.automation_mode === 'agent') ? form.initial_prompt : undefined,
      ai_managed: form.automation_mode === 'cli' ? form.ai_managed : undefined,
      ai_prompt: (form.automation_mode === 'cli' || form.automation_mode === 'agent') ? form.ai_prompt : undefined,
      ai_end_condition: (form.automation_mode === 'cli' || form.automation_mode === 'agent') ? form.ai_end_condition : undefined,
      ai_error_handling: (form.automation_mode === 'cli' || form.automation_mode === 'agent') ? form.ai_error_handling : undefined
    })
    message.success('任务已更新')
    emit('saved', updated)
    close()
  } catch (error: any) {
    message.error(error.response?.data?.error || '更新任务失败')
  } finally {
    saving.value = false
  }
}
</script>
