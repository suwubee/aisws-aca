<template>
  <n-modal
    v-model:show="showModal"
    preset="dialog"
    :title="mode === 'create' ? '添加服务器' : '编辑服务器'"
    style="width: 640px"
  >
    <n-form
      ref="formRef"
      :model="form"
      :rules="rules"
      label-placement="left"
      label-width="90"
    >
      <n-form-item label="名称" path="name">
        <n-input v-model:value="form.name" placeholder="例如: 生产环境A" />
      </n-form-item>

      <n-form-item label="主机" path="host">
        <n-input v-model:value="form.host" placeholder="例如: 192.168.1.10 / example.com" />
      </n-form-item>

      <n-form-item label="端口" path="port">
        <n-input-number v-model:value="form.port" :min="1" :max="65535" placeholder="默认 22" style="width: 100%" />
      </n-form-item>

      <n-form-item label="用户名" path="username">
        <n-input v-model:value="form.username" placeholder="例如: root / ubuntu" />
      </n-form-item>

      <n-form-item label="分组" path="group_id">
        <n-select
          v-model:value="form.group_id"
          :options="groupOptions"
          clearable
          placeholder="选择分组（可选）"
        />
      </n-form-item>

      <n-divider />

      <n-form-item label="认证方式" path="auth_type">
        <n-radio-group v-model:value="form.auth_type">
          <n-radio value="password">密码</n-radio>
          <n-radio value="key">密钥</n-radio>
        </n-radio-group>
      </n-form-item>

      <n-form-item v-if="form.auth_type === 'password'" label="密码" path="password">
        <n-input
          v-model:value="form.password"
          type="password"
          show-password-on="click"
          :placeholder="mode === 'edit' ? '不修改请留空' : '请输入密码'"
        />
      </n-form-item>

      <template v-else>
        <n-form-item label="私钥文件" path="keyFile">
          <n-space vertical style="width: 100%">
            <n-upload
              :default-upload="false"
              :max="1"
              :file-list="keyFileList"
              accept=".pem,.key,.txt"
              @update:file-list="handleKeyFileListUpdate"
            >
              <n-button>选择私钥文件</n-button>
            </n-upload>
            <n-text depth="3">编辑时不选择文件表示保留现有私钥</n-text>
          </n-space>
        </n-form-item>

        <n-form-item label="私钥口令" path="passphrase">
          <n-input
            v-model:value="form.passphrase"
            type="password"
            show-password-on="click"
            placeholder="可选：如果私钥有口令请输入"
          />
        </n-form-item>
      </template>
    </n-form>

    <template #action>
      <n-button :disabled="saving" @click="close">取消</n-button>
      <n-button type="primary" :loading="saving" @click="submit">
        {{ mode === 'create' ? '创建' : '保存' }}
      </n-button>
    </template>
  </n-modal>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useMessage } from 'naive-ui'
import type { FormInst, FormRules, UploadFileInfo } from 'naive-ui'
import type { CreateSSHServerRequest, SSHAuthType, SSHServer, ServerGroup, UpdateSSHServerRequest } from '@/api/server'
import { createServer, updateServer, uploadKey } from '@/api/server'

const props = defineProps<{
  show: boolean
  mode: 'create' | 'edit'
  server: SSHServer | null
  groups: ServerGroup[]
}>()

const emit = defineEmits<{
  'update:show': [show: boolean]
  saved: [server: SSHServer]
}>()

const message = useMessage()

const showModal = computed({
  get: () => props.show,
  set: (value: boolean) => emit('update:show', value)
})

const saving = ref(false)
const formRef = ref<FormInst | null>(null)
const keyFileList = ref<UploadFileInfo[]>([])

const form = reactive({
  name: '',
  host: '',
  port: 22 as number | null,
  username: '',
  group_id: null as string | null,
  auth_type: 'password' as SSHAuthType,
  password: '',
  keyFile: null as File | null,
  passphrase: ''
})

const groupOptions = computed(() =>
  props.groups.map(g => ({ label: g.name, value: g.id }))
)

function normalizeAuthType(value: string | null | undefined): SSHAuthType {
  return value === 'key' ? 'key' : 'password'
}

function resetForm() {
  form.name = ''
  form.host = ''
  form.port = 22
  form.username = ''
  form.group_id = null
  form.auth_type = 'password'
  form.password = ''
  form.keyFile = null
  form.passphrase = ''
  keyFileList.value = []
}

function syncFormFromServer(server: SSHServer | null) {
  if (!server) {
    resetForm()
    return
  }

  form.name = server.name || ''
  form.host = server.host || ''
  form.port = typeof server.port === 'number' ? server.port : 22
  form.username = server.username || ''
  form.group_id = server.group_id || null
  form.auth_type = normalizeAuthType(server.auth_type)

  form.password = ''
  form.keyFile = null
  form.passphrase = ''
  keyFileList.value = []
}

watch(
  () => [props.show, props.mode, props.server] as const,
  ([show, mode, server]) => {
    if (!show) return
    if (mode === 'edit') {
      syncFormFromServer(server)
    } else {
      resetForm()
    }
  },
  { immediate: true }
)

watch(
  () => form.auth_type,
  (value) => {
    if (value === 'password') {
      form.keyFile = null
      form.passphrase = ''
      keyFileList.value = []
    } else {
      form.password = ''
    }
  }
)

function handleKeyFileListUpdate(files: UploadFileInfo[]) {
  const last = files.slice(-1)
  keyFileList.value = last
  form.keyFile = last[0]?.file || null
}

const rules: FormRules = {
  name: { required: true, message: '请输入名称' },
  host: { required: true, message: '请输入主机' },
  port: [
    { required: true, type: 'number', message: '请输入端口' },
    {
      validator: (_rule, value: number | null) => {
        if (typeof value !== 'number') return new Error('请输入端口')
        if (value < 1 || value > 65535) return new Error('端口范围应为 1-65535')
        return true
      },
      trigger: ['blur', 'change']
    }
  ],
  username: { required: true, message: '请输入用户名' },
  auth_type: { required: true, message: '请选择认证方式' },
  password: {
    validator: () => {
      if (form.auth_type !== 'password') return true
      if (props.mode === 'edit' && props.server && normalizeAuthType(props.server.auth_type) === 'password') {
        return true
      }
      if (!form.password.trim()) return new Error('请输入密码')
      return true
    },
    trigger: ['blur', 'input']
  },
  keyFile: {
    validator: () => {
      if (form.auth_type !== 'key') return true
      if (form.keyFile) return true
      if (props.mode === 'edit' && props.server && normalizeAuthType(props.server.auth_type) === 'key') {
        return true
      }
      return new Error('请选择私钥文件')
    },
    trigger: ['change']
  }
}

function close() {
  showModal.value = false
}

async function submit() {
  if (saving.value) return

  try {
    await formRef.value?.validate()
  } catch {
    return
  }

  saving.value = true
  try {
    if (props.mode === 'create') {
      const payload: CreateSSHServerRequest = {
        name: form.name.trim(),
        host: form.host.trim(),
        port: form.port || 22,
        username: form.username.trim(),
        auth_type: form.auth_type
      }

      if (form.group_id) {
        payload.group_id = form.group_id
      }

      if (form.auth_type === 'password') {
        payload.password = form.password
      } else {
        if (!form.keyFile) {
          message.warning('请选择私钥文件')
          return
        }
        payload.private_key = await form.keyFile.text()
        if (form.passphrase.trim()) {
          payload.passphrase = form.passphrase.trim()
        }
      }

      const { data } = await createServer(payload)
      message.success('服务器创建成功')
      emit('saved', data.item as SSHServer)
      close()
      return
    }

    if (!props.server) {
      message.error('未选择要编辑的服务器')
      return
    }

    const updates: UpdateSSHServerRequest = {}
    const current = props.server

    const name = form.name.trim()
    const host = form.host.trim()
    const username = form.username.trim()
    const port = form.port || 22

    if (name && name !== current.name) updates.name = name
    if (host && host !== current.host) updates.host = host
    if (username && username !== current.username) updates.username = username
    if (port !== current.port) updates.port = port

    const nextGroup = form.group_id || null
    const prevGroup = current.group_id || null
    if (nextGroup !== prevGroup) {
      updates.group_id = nextGroup || ''
    }

    const prevAuth = normalizeAuthType(current.auth_type)
    const nextAuth = normalizeAuthType(form.auth_type)

    if (nextAuth === 'password') {
      if (prevAuth !== 'password') {
        updates.auth_type = 'password'
      }
      if (form.password.trim()) {
        updates.password = form.password
      }
    }

    let latest: SSHServer | null = null

    if (nextAuth === 'key' && form.keyFile) {
      const { data } = await uploadKey(current.id, form.keyFile, form.passphrase)
      latest = data.item as SSHServer
    }

    if (Object.keys(updates).length > 0) {
      const { data } = await updateServer(current.id, updates)
      latest = data.item as SSHServer
    }

    message.success('服务器已更新')
    emit('saved', latest || current)
    close()
  } catch (e: any) {
    message.error(e.response?.data?.error || (props.mode === 'create' ? '创建服务器失败' : '更新服务器失败'))
  } finally {
    saving.value = false
  }
}
</script>
