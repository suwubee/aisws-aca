<template>
  <n-modal v-model:show="showModal" preset="dialog" title="编辑任务" style="width: min(600px, 94vw)">
    <n-form :model="form">
      <n-form-item label="标题" required>
        <n-input v-model:value="form.title" placeholder="任务标题" />
      </n-form-item>
      <n-form-item label="描述">
        <n-input v-model:value="form.description" type="textarea" placeholder="任务描述" />
      </n-form-item>
      <n-form-item label="优先级">
        <n-radio-group v-model:value="form.priority">
          <n-radio :value="0">低</n-radio>
          <n-radio :value="1">中</n-radio>
          <n-radio :value="2">高</n-radio>
          <n-radio :value="3">紧急</n-radio>
        </n-radio-group>
      </n-form-item>

      <n-divider>自动化配置（可选）</n-divider>

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

      <n-divider>AI托管配置（可选）</n-divider>

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

const showModal = computed({
  get: () => props.show,
  set: (value: boolean) => emit('update:show', value)
})

const saving = ref(false)
const form = reactive({
  title: '',
  description: '',
  priority: 1,
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

const errorHandlingOptions = [
  { label: '暂停等待', value: 'pause' },
  { label: '自动重试', value: 'retry' },
  { label: '标记失败', value: 'fail' }
]

function syncFormFromTask(task: Task | null) {
  form.title = task?.title ?? ''
  form.description = task?.description ?? ''
  const priority = typeof task?.priority === 'number' ? task.priority : 1
  form.priority = Math.min(3, Math.max(0, priority))
  form.work_dir = task?.work_dir ?? ''
  form.cli_type = task?.cli_type || null
  form.initial_prompt = task?.initial_prompt ?? ''
  // AI托管配置
  form.ai_managed = task?.ai_managed ?? false
  form.ai_prompt = task?.ai_prompt ?? ''
  form.ai_end_condition = task?.ai_end_condition ?? ''
  form.ai_error_handling = task?.ai_error_handling || 'pause'
}

watch(
  () => [props.show, props.task] as const,
  ([show, task]) => {
    if (show) {
      syncFormFromTask(task)
    }
  },
  { immediate: true }
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
      priority: form.priority,
      work_dir: form.work_dir,
      cli_type: form.cli_type || '',
      initial_prompt: form.initial_prompt,
      ai_managed: form.ai_managed,
      ai_prompt: form.ai_prompt,
      ai_end_condition: form.ai_end_condition,
      ai_error_handling: form.ai_error_handling
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
