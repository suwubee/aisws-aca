<template>
  <n-card size="small">
    <div class="user-management">
      <div class="toolbar">
        <n-space justify="space-between" align="center">
          <div class="toolbar-left">
            <n-text depth="3">仅管理员可查看与管理用户</n-text>
          </div>
          <n-space>
            <n-button size="small" :loading="loading" @click="fetchUsers" :disabled="!isAdmin">
              刷新
            </n-button>
            <n-button size="small" type="primary" @click="openCreateModal" :disabled="!isAdmin">
              创建用户
            </n-button>
          </n-space>
        </n-space>
      </div>

      <n-alert v-if="!isAdmin" type="warning" :bordered="false">
        当前账号无权限访问用户管理功能
      </n-alert>

      <n-data-table
        v-else
        :columns="columns"
        :data="users"
        :loading="loading"
        :row-key="(row: User) => row.id"
        size="small"
      />

      <!-- Create User -->
      <n-modal
        v-model:show="showCreateModal"
        preset="dialog"
        title="创建用户"
        positive-text="创建"
        negative-text="取消"
        style="width: 520px"
        @positive-click="createUser"
      >
        <n-form ref="createFormRef" :model="createForm" :rules="createRules" label-placement="left" label-width="90">
          <n-form-item label="用户名" path="username">
            <n-input v-model:value="createForm.username" placeholder="例如: alice" />
          </n-form-item>
          <n-form-item label="邮箱" path="email">
            <n-input v-model:value="createForm.email" placeholder="例如: alice@example.com" />
          </n-form-item>
          <n-form-item label="密码" path="password">
            <n-input
              v-model:value="createForm.password"
              type="password"
              show-password-on="click"
              placeholder="至少6位"
            />
          </n-form-item>
          <n-form-item label="角色" path="role">
            <n-select v-model:value="createForm.role" :options="roleOptions" />
          </n-form-item>
          <n-form-item label="状态" path="status">
            <n-select v-model:value="createForm.status" :options="statusOptions" />
          </n-form-item>
        </n-form>
      </n-modal>

      <!-- Edit User -->
      <n-modal
        v-model:show="showEditModal"
        preset="dialog"
        title="编辑用户"
        positive-text="保存"
        negative-text="取消"
        style="width: 520px"
        @positive-click="saveUser"
      >
        <n-form ref="editFormRef" :model="editForm" :rules="editRules" label-placement="left" label-width="90">
          <n-form-item label="用户名">
            <n-input :value="editingUser?.username || ''" disabled />
          </n-form-item>
          <n-form-item label="邮箱">
            <n-input :value="editingUser?.email || ''" disabled />
          </n-form-item>
          <n-form-item label="角色" path="role">
            <n-select v-model:value="editForm.role" :options="roleOptions" />
          </n-form-item>
          <n-form-item label="状态" path="status">
            <n-select v-model:value="editForm.status" :options="statusOptions" />
          </n-form-item>
        </n-form>
      </n-modal>
    </div>
  </n-card>
</template>

<script setup lang="ts">
import { computed, h, reactive, ref, watch } from 'vue'
import {
  NAlert,
  NButton,
  NCard,
  NDataTable,
  NForm,
  NFormItem,
  NInput,
  NModal,
  NPopconfirm,
  NSelect,
  NSpace,
  NTag,
  NText,
  useMessage
} from 'naive-ui'
import type { DataTableColumns, FormInst, FormRules } from 'naive-ui'
import { authApi, userApi, type RegisterUserRequest, type UpdateUserRequest, type User, type UserRole, type UserStatus } from '@/api'
import { useAuthStore } from '@/stores/auth'

const message = useMessage()
const authStore = useAuthStore()

const isAdmin = computed(() => authStore.user?.role === 'admin')

const loading = ref(false)
const users = ref<User[]>([])

const roleOptions = [
  { label: '管理员', value: 'admin' },
  { label: '普通用户', value: 'user' },
  { label: '只读用户', value: 'viewer' }
]

const statusOptions = [
  { label: '启用', value: 'active' },
  { label: '禁用', value: 'disabled' }
]

function roleTagType(role: string) {
  if (role === 'admin') return 'success'
  if (role === 'viewer') return 'warning'
  return 'default'
}

function statusTagType(status: string) {
  if (status === 'active') return 'success'
  if (status === 'disabled') return 'error'
  return 'default'
}

function normalizeRole(role: string | null | undefined): UserRole {
  if (role === 'admin' || role === 'user' || role === 'viewer') return role
  return 'user'
}

function normalizeStatus(status: string | null | undefined): UserStatus {
  if (status === 'active' || status === 'disabled') return status
  return 'active'
}

function formatDate(value: string | null) {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '—'
  return date.toLocaleString('zh-CN')
}

const columns: DataTableColumns<User> = [
  { title: '用户名', key: 'username', width: 140 },
  { title: '邮箱', key: 'email', ellipsis: { tooltip: true } },
  {
    title: '角色',
    key: 'role',
    width: 100,
    render: (row) => h(NTag, { size: 'small', bordered: false, type: roleTagType(row.role) }, () => row.role)
  },
  {
    title: '状态',
    key: 'status',
    width: 90,
    render: (row) => h(NTag, { size: 'small', bordered: false, type: statusTagType(row.status) }, () => row.status === 'active' ? '启用' : '禁用')
  },
  { title: '最近登录', key: 'last_login_at', width: 160, render: (row) => formatDate(row.last_login_at) },
  {
    title: '操作',
    key: 'actions',
    width: 180,
    render: (row) => h(NSpace, { size: 'small' }, () => [
      h(NButton, { size: 'tiny', quaternary: true, onClick: () => openEditModal(row) }, () => '编辑'),
      h(NPopconfirm, { onPositiveClick: () => toggleStatus(row) }, {
        trigger: () => h(NButton, {
          size: 'tiny',
          quaternary: true,
          type: row.status === 'active' ? 'error' : 'success'
        }, () => row.status === 'active' ? '禁用' : '启用'),
        default: () => row.status === 'active' ? `确定禁用用户「${row.username}」吗？` : `确定启用用户「${row.username}」吗？`
      })
    ])
  }
]

async function fetchUsers() {
  if (!isAdmin.value) return
  loading.value = true
  try {
    const { data } = await userApi.list()
    users.value = data.items || []
  } catch (e: any) {
    message.error(e.response?.data?.error || '获取用户列表失败')
  } finally {
    loading.value = false
  }
}

// ===== Create User =====
const showCreateModal = ref(false)
const createFormRef = ref<FormInst | null>(null)

const createForm = reactive<RegisterUserRequest>({
  username: '',
  email: '',
  password: '',
  role: 'user',
  status: 'active'
})

const createRules: FormRules = {
  username: { required: true, message: '请输入用户名' },
  email: { required: true, message: '请输入邮箱' },
  password: [
    { required: true, message: '请输入密码' },
    { min: 6, message: '密码至少6位' }
  ],
  role: { required: true, message: '请选择角色' },
  status: { required: true, message: '请选择状态' }
}

function resetCreateForm() {
  Object.assign(createForm, {
    username: '',
    email: '',
    password: '',
    role: 'user',
    status: 'active'
  })
}

function openCreateModal() {
  resetCreateForm()
  showCreateModal.value = true
}

async function createUser() {
  if (!isAdmin.value) return false
  try {
    await createFormRef.value?.validate()
  } catch {
    return false
  }

  try {
    const payload: RegisterUserRequest = {
      username: createForm.username.trim(),
      email: createForm.email.trim(),
      password: createForm.password,
      role: createForm.role,
      status: createForm.status
    }
    const { data } = await authApi.register(payload)
    if (data?.item?.id) {
      users.value = [data.item as User, ...users.value]
    } else {
      await fetchUsers()
    }
    message.success('用户创建成功')
    showCreateModal.value = false
    resetCreateForm()
  } catch (e: any) {
    message.error(e.response?.data?.error || '创建用户失败')
    return false
  }
}

// ===== Edit User =====
const showEditModal = ref(false)
const editingUser = ref<User | null>(null)
const editFormRef = ref<FormInst | null>(null)

const editForm = reactive<Required<UpdateUserRequest>>({
  role: 'user',
  status: 'active'
})

const editRules: FormRules = {
  role: { required: true, message: '请选择角色' },
  status: { required: true, message: '请选择状态' }
}

function openEditModal(user: User) {
  editingUser.value = user
  editForm.role = normalizeRole(user.role)
  editForm.status = normalizeStatus(user.status)
  showEditModal.value = true
}

async function saveUser() {
  if (!isAdmin.value) return false
  if (!editingUser.value) {
    message.error('未选择要编辑的用户')
    return false
  }

  try {
    await editFormRef.value?.validate()
  } catch {
    return false
  }

  try {
    const payload: UpdateUserRequest = {
      role: editForm.role,
      status: editForm.status
    }
    const { data } = await userApi.update(editingUser.value.id, payload)
    const updated: User | null = data?.item?.id ? (data.item as User) : null

    const idx = users.value.findIndex(u => u.id === editingUser.value?.id)
    if (idx >= 0) {
      users.value[idx] = updated || { ...users.value[idx], ...payload }
    }

    message.success('用户已更新')
    showEditModal.value = false
    editingUser.value = null
  } catch (e: any) {
    message.error(e.response?.data?.error || '更新用户失败')
    return false
  }
}

async function toggleStatus(user: User) {
  if (!isAdmin.value) return
  const nextStatus: UserStatus = user.status === 'active' ? 'disabled' : 'active'
  try {
    const payload: UpdateUserRequest = { status: nextStatus }
    const { data } = await userApi.update(user.id, payload)
    const updated: User | null = data?.item?.id ? (data.item as User) : null
    const idx = users.value.findIndex(u => u.id === user.id)
    if (idx >= 0) {
      users.value[idx] = updated || { ...users.value[idx], ...payload }
    }
    message.success(nextStatus === 'active' ? '用户已启用' : '用户已禁用')
  } catch (e: any) {
    message.error(e.response?.data?.error || '更新用户状态失败')
  }
}

watch(isAdmin, (value) => {
  if (value) fetchUsers()
}, { immediate: true })
</script>

<style scoped>
.toolbar {
  margin-bottom: 12px;
}
</style>
