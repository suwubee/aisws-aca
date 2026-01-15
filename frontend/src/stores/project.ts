import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import type { Project } from '@/api/project'
import { getProjects } from '@/api/project'
import type { ProjectGroup } from '@/api/project-group'
import { getProjectGroups } from '@/api/project-group'

export const useProjectStore = defineStore('project', () => {
  const projects = ref<Project[]>([])
  const groups = ref<ProjectGroup[]>([])

  const loadingProjects = ref(false)
  const loadingGroups = ref(false)

  const loadedProjects = ref(false)
  const loadedGroups = ref(false)

  const groupNameMap = computed(() => {
    const map = new Map<string, string>()
    groups.value.forEach(g => map.set(g.id, g.name))
    return map
  })

  const projectNameMap = computed(() => {
    const map = new Map<string, string>()
    projects.value.forEach(p => map.set(p.id, p.name))
    return map
  })

  const projectOptions = computed(() =>
    projects.value.map(p => {
      const groupName = p.group_id ? groupNameMap.value.get(p.group_id) : null
      return { label: groupName ? `${groupName} / ${p.name}` : p.name, value: p.id }
    })
  )

  const projectGroupOptions = computed(() =>
    groups.value.map(g => ({ label: g.name, value: g.id }))
  )

  function getProjectName(projectId?: string | null) {
    if (!projectId) return null
    return projectNameMap.value.get(projectId) || null
  }

  function getProjectGroupName(groupId?: string | null) {
    if (!groupId) return null
    return groupNameMap.value.get(groupId) || null
  }

  async function fetchProjects(options?: { force?: boolean }) {
    if (loadingProjects.value) return
    if (loadedProjects.value && !options?.force) return

    loadingProjects.value = true
    try {
      const { data } = await getProjects()
      projects.value = data.items || []
      loadedProjects.value = true
    } finally {
      loadingProjects.value = false
    }
  }

  async function fetchProjectGroups(options?: { force?: boolean }) {
    if (loadingGroups.value) return
    if (loadedGroups.value && !options?.force) return

    loadingGroups.value = true
    try {
      const { data } = await getProjectGroups()
      groups.value = data.items || []
      loadedGroups.value = true
    } finally {
      loadingGroups.value = false
    }
  }

  async function fetchAll(options?: { force?: boolean }) {
    await Promise.all([
      fetchProjectGroups(options),
      fetchProjects(options)
    ])
  }

  return {
    projects,
    groups,
    loadingProjects,
    loadingGroups,
    loadedProjects,
    loadedGroups,
    groupNameMap,
    projectOptions,
    projectGroupOptions,
    getProjectName,
    getProjectGroupName,
    fetchProjects,
    fetchProjectGroups,
    fetchAll
  }
})

