<script setup>
import { computed, getCurrentInstance, onMounted, ref } from 'vue'
import api, { copyText } from '../api'
import Modal from '../components/Modal.vue'
import PageState from '../components/PageState.vue'
import HeaderOverrideEditor from '../components/HeaderOverrideEditor.vue'
import BodyOverrideEditor from '../components/BodyOverrideEditor.vue'
import ActionMenu from '../components/ActionMenu.vue'

const { proxy } = getCurrentInstance()
const notify = (message, type = 'info', duration) => proxy?.$toast?.add(message, type, duration)

const channels = ref([])
const channelTypes = ref([])
const protocols = ref([])
const loading = ref(true)
const loadError = ref('')
const metadataLoading = ref(false)

const showEditor = ref(false)
const probing = ref(false)
const saving = ref(false)
const editorError = ref('')
const editorTab = ref('connection')
const headerValidation = ref({ valid: true, error: '', allowedCount: 0, ignored: [] })
const bodyValidation = ref({ valid: true, error: '', keyCount: 0, ignored: [] })
const models = ref([])
const rules = ref([])
const newModelName = ref('')
const testing = ref({})
const testResults = ref({})

const batchTesting = ref(false)
const batchDone = ref(0)
const batchTotal = ref(0)
const batchSummary = ref(null)

const checkupLoadingId = ref(null)
const showCheckup = ref(false)
const checkupChannelName = ref('')
const checkupResults = ref([])
const checkupSummary = ref(null)

const expandedIds = ref(new Set())
const selectedIds = ref(new Set())
const modelMutating = ref(new Set())
const globalTestPrompt = ref("Say 'hi' in one word.")
const bulkDeleting = ref(false)

const togglingIds = ref(new Set())
const deletingIds = ref(new Set())
const resettingIds = ref(new Set())
const dragIndex = ref(null)
const dropIndex = ref(null)
const reordering = ref(false)

const blank = () => ({
  id: 0,
  name: '',
  type: 1,
  base_url: '',
  key: '',
  group: 'default',
  header_override: '',
  body_override: '',
  test_prompt: '',
  priority: 0,
  weight: 1,
  status: 1,
})
const form = ref(blank())

const sortedChannels = computed(() => channels.value)
const allSelected = computed(() => channels.value.length > 0 && selectedIds.value.size === channels.value.length)
const enabledCount = computed(() => models.value.filter((model) => model.enabled && model.name.trim()).length)
const hasModelTesting = computed(() => Object.values(testing.value).some(Boolean))
const editorBusy = computed(() => saving.value || probing.value || batchTesting.value || hasModelTesting.value)
const canSave = computed(() => Boolean(
  form.value.name.trim()
  && form.value.base_url.trim()
  && String(form.value.key || '').trim()
  && enabledCount.value > 0
  && headerValidation.value.valid
  && bodyValidation.value.valid
))
const customHeaderCount = computed(() => headerValidation.value.valid ? headerValidation.value.allowedCount : 0)
const checkupRate = computed(() => {
  const summary = checkupSummary.value
  if (!summary?.total) return 0
  return Math.round((summary.success / summary.total) * 100)
})
const testRecordRows = computed(() => models.value
  .map((model) => ({ model, result: testResults.value[model.name] }))
  .filter((row) => row.result || testing.value[row.model.name]))

function typeName(type) {
  const found = channelTypes.value.find((item) => item.value === type)
  return found ? found.name : String(type)
}

function modelCount(channel) {
  if (Array.isArray(channel._models)) return channel._models.length
  if (channel.model_configs) {
    try {
      const list = JSON.parse(channel.model_configs)
      if (Array.isArray(list)) return list.length
    } catch {
      // 兼容旧 models 字段。
    }
  }
  return (channel.models || '').split(',').map((item) => item.trim()).filter(Boolean).length
}

function modelHealth(channel, item) {
  return channel?.model_health?.[item?.name] || null
}

function healthTotal(health) {
  return Number(health?.total) || 0
}

function hasHealth(health) {
  return healthTotal(health) > 0
}

function healthPercent(health) {
  if (!hasHealth(health)) return 0
  return Math.round((Number(health.availability) || 0) * 10) / 10
}

function healthClass(health) {
  if (!hasHealth(health)) return ''
  const percent = healthPercent(health)
  if (percent >= 95) return 'chip-run'
  if (percent >= 70) return 'chip-test'
  return 'chip-trip'
}

function healthText(health) {
  if (!hasHealth(health)) return '未调用'
  return `${healthPercent(health)}% · ${Number(health.success) || 0}/${healthTotal(health)}`
}

function healthTitle(health) {
  if (!hasHealth(health)) return '尚无真实调用日志'
  const parts = [`成功 ${Number(health.success) || 0}`, `失败 ${Number(health.failed) || 0}`]
  if (health.last_error) parts.push(health.last_error)
  return parts.join(' · ')
}

function channelHealth(channel) {
  const stats = Object.values(channel?.model_health || {}).filter(Boolean)
  const called = stats.filter(hasHealth)
  if (!called.length) return null
  const total = called.reduce((sum, item) => sum + healthTotal(item), 0)
  const success = called.reduce((sum, item) => sum + (Number(item.success) || 0), 0)
  const failed = called.reduce((sum, item) => sum + (Number(item.failed) || 0), 0)
  return { total, success, failed, availability: total ? (success / total) * 100 : 0 }
}

function breakerState(channel) {
  if (checkupLoadingId.value === channel.id) return 'test'
  if (channel.status !== 1) return 'off'
  if (channel.cooldown_until && channel.cooldown_until > Date.now()) return 'trip'
  return 'run'
}

function breakerText(channel) {
  return { run: '运行中', test: '测试中', trip: '已熔断', off: '已停用' }[breakerState(channel)]
}

function validateHeaders(action) {
  if (!headerValidation.value.valid) {
    editorTab.value = 'routing'
    editorError.value = `无法${action}：${headerValidation.value.error}`
    notify(editorError.value, 'warn')
    return false
  }
  if (!bodyValidation.value.valid) {
    editorTab.value = 'routing'
    editorError.value = `无法${action}：${bodyValidation.value.error}`
    notify(editorError.value, 'warn')
    return false
  }
  return true
}

function updateSet(target, value, active) {
  const next = new Set(target.value)
  if (active) next.add(value)
  else next.delete(value)
  target.value = next
}

function maskedKey(key) {
  const value = String(key || '')
  if (!value) return '未配置'
  if (value.length <= 10) return `${value.slice(0, 3)}••••`
  return `${value.slice(0, 6)}••••${value.slice(-4)}`
}

async function copyKey(channel) {
  if (!channel.key) return
  const copied = await copyText(channel.key)
  if (copied) notify(`已复制「${channel.name}」的上游 Key`, 'success')
  else notify('浏览器阻止了剪贴板写入，请手动复制 Key', 'warn')
}

async function load() {
  loading.value = true
  loadError.value = ''
  try {
    const data = (await api.get('/channels')) || []
    channels.value = data.map((channel) => ({ ...channel, _models: parseModels(channel) }))
    selectedIds.value = new Set([...selectedIds.value].filter((id) => channels.value.some((channel) => channel.id === id)))
  } catch (error) {
    loadError.value = error.message || '渠道清单加载失败'
    notify(`加载失败：${loadError.value}`, 'error')
  } finally {
    loading.value = false
  }
}

async function loadMeta() {
  if (metadataLoading.value) return
  metadataLoading.value = true
  try {
    const [types, protocolList, promptData] = await Promise.all([
      api.get('/channel-types'),
      api.get('/protocols'),
      api.get('/settings/test-prompt'),
    ])
    channelTypes.value = types || []
    protocols.value = protocolList || []
    globalTestPrompt.value = promptData?.prompt || globalTestPrompt.value
  } catch (error) {
    notify(`加载元数据失败：${error.message}`, 'error')
  } finally {
    metadataLoading.value = false
  }
}

function parseModels(channel) {
  if (channel.model_configs) {
    try {
      const list = JSON.parse(channel.model_configs)
      if (Array.isArray(list)) {
        return list.map((model) => ({
          name: model.name || '',
          enabled: model.enabled !== false,
          protocol: model.protocol || '',
          upstream: model.upstream || '',
          input: model.input || 0,
          output: model.output || 0,
        }))
      }
    } catch {
      // 兼容旧 models 字段。
    }
  }
  return (channel.models || '').split(',').map((item) => item.trim()).filter(Boolean)
    .map((name) => ({ name, enabled: true, protocol: '', upstream: '', input: 0, output: 0 }))
}

function parseRules(channel) {
  if (channel.protocol_rules) {
    try {
      const list = JSON.parse(channel.protocol_rules)
      if (Array.isArray(list)) {
        return list.map((rule) => ({
          pattern: rule.pattern || '',
          protocol: rule.protocol || 'anthropic',
        }))
      }
    } catch {
      // 无效旧值按空规则处理。
    }
  }
  return []
}

function resetEditorState() {
  editorError.value = ''
  editorTab.value = 'connection'
  newModelName.value = ''
  testing.value = {}
  testResults.value = {}
  batchSummary.value = null
  batchDone.value = 0
  batchTotal.value = 0
}

function openCreate() {
  form.value = blank()
  models.value = []
  rules.value = []
  resetEditorState()
  const type = channelTypes.value.find((item) => item.value === form.value.type)
  if (type) form.value.base_url = type.default_base_url
  showEditor.value = true
}

function openEdit(channel) {
  form.value = { ...blank(), ...channel, key: channel.key || '' }
  models.value = (Array.isArray(channel._models) ? channel._models : parseModels(channel)).map((item) => ({ ...item }))
  rules.value = parseRules(channel)
  resetEditorState()
  showEditor.value = true
}

function closeEditor() {
  if (editorBusy.value) return
  showEditor.value = false
}

function onTypeChange() {
  const type = channelTypes.value.find((item) => item.value === form.value.type)
  if (type && !form.value.base_url) form.value.base_url = type.default_base_url
}

function addModel() {
  const name = newModelName.value.trim()
  if (!name) return
  if (models.value.some((model) => model.name === name)) {
    notify('模型已存在', 'warn')
    return
  }
  models.value.unshift({ name, enabled: true, protocol: '', upstream: '', input: 0, output: 0 })
  newModelName.value = ''
}

function removeModel(index, model) {
  if (testing.value[model?.name] || batchTesting.value) return
  models.value.splice(index, 1)
  if (model?.name) {
    const nextResults = { ...testResults.value }
    const nextTesting = { ...testing.value }
    delete nextResults[model.name]
    delete nextTesting[model.name]
    testResults.value = nextResults
    testing.value = nextTesting
  }
}

function testPayload() {
  return {
    type: form.value.type,
    base_url: form.value.base_url,
    key: form.value.key,
    group: form.value.group || 'default',
    protocol_rules: JSON.stringify(rules.value.filter((rule) => rule.pattern.trim() && rule.protocol)),
    header_override: form.value.header_override || '',
    body_override: form.value.body_override || '',
    test_prompt: form.value.test_prompt || '',
    prompt: form.value.test_prompt?.trim() || globalTestPrompt.value,
  }
}

async function testModel(model) {
  const name = model.name.trim()
  if (!name || testing.value[name] || batchTesting.value || saving.value) return
  if (!validateHeaders('测试模型')) return
  if (!form.value.base_url) {
    notify('请先填写 Base URL', 'warn')
    return
  }
  if (!form.value.key) {
    notify('请先填写 API Key', 'warn')
    return
  }

  testing.value = { ...testing.value, [name]: true }
  testResults.value = {
    ...testResults.value,
    [name]: { model: name, pending: true, success: false },
  }
  try {
    const result = await api.post('/channels/test', {
      ...testPayload(),
      model_configs: JSON.stringify([{
        name,
        enabled: true,
        protocol: model.protocol || '',
        upstream: model.upstream || '',
      }]),
      model: name,
    })
    testResults.value = { ...testResults.value, [name]: { ...result, model: name } }
    notify(result.success ? `模型 ${name} 连通正常` : `模型 ${name} 测试失败`, result.success ? 'success' : 'error')
  } catch (error) {
    testResults.value = {
      ...testResults.value,
      [name]: { model: name, success: false, error: error.message || '请求失败' },
    }
    notify(`测试失败：${error.message || '请求失败'}`, 'error')
  } finally {
    testing.value = { ...testing.value, [name]: false }
  }
}

async function testAllInModal() {
  if (batchTesting.value || saving.value || probing.value || hasModelTesting.value) return
  if (!validateHeaders('批量测试')) return
  if (!form.value.base_url) {
    notify('请先填写 Base URL', 'warn')
    return
  }
  if (!form.value.key) {
    notify('请先填写 API Key', 'warn')
    return
  }
  const enabled = models.value.filter((model) => model.enabled && model.name.trim())
  if (!enabled.length) {
    notify('没有可测试的启用模型', 'warn')
    return
  }

  batchTesting.value = true
  batchTotal.value = enabled.length
  batchDone.value = 0
  batchSummary.value = null
  const pending = { ...testResults.value }
  enabled.forEach((model) => {
    pending[model.name.trim()] = { model: model.name.trim(), pending: true, success: false }
  })
  testResults.value = pending

  try {
    const response = await api.post('/channels/test-batch', {
      ...testPayload(),
      model_configs: JSON.stringify(enabled.map((model) => ({
        name: model.name.trim(),
        enabled: true,
        protocol: model.protocol || '',
        upstream: model.upstream || '',
      }))),
      models: enabled.map((model) => model.name.trim()),
    })
    const results = response.results || []
    const merged = { ...testResults.value }
    results.forEach((result) => {
      if (result?.model) merged[result.model] = result
    })
    testResults.value = merged
    batchDone.value = results.length
    batchSummary.value = response.summary || null
    if (response.summary) {
      const { success, failed } = response.summary
      notify(
        failed === 0 ? `全部 ${success} 个模型连通正常` : `测试完成：成功 ${success} · 失败 ${failed}`,
        failed === 0 ? 'success' : 'warn',
      )
    }
  } catch (error) {
    enabled.forEach((model) => {
      const name = model.name.trim()
      if (testResults.value[name]?.pending) {
        testResults.value[name] = { model: name, success: false, error: error.message || '请求失败' }
      }
    })
    testResults.value = { ...testResults.value }
    notify(`批量测试失败：${error.message || '请求失败'}`, 'error')
  } finally {
    batchTesting.value = false
  }
}

async function checkupChannel(channel) {
  if (checkupLoadingId.value !== null) return
  checkupLoadingId.value = channel.id
  checkupChannelName.value = channel.name
  checkupResults.value = []
  checkupSummary.value = null
  try {
    const response = await api.post(`/channels/${channel.id}/test-all`, {
      prompt: channel.test_prompt?.trim() || globalTestPrompt.value,
    })
    checkupResults.value = response.results || []
    checkupSummary.value = response.summary || null
    showCheckup.value = true
    if (response.summary) {
      const { success, failed } = response.summary
      notify(
        failed === 0
          ? `「${channel.name}」全部 ${success} 个模型连通正常`
          : `「${channel.name}」体检：成功 ${success} · 失败 ${failed}`,
        failed === 0 ? 'success' : 'warn',
      )
    }
  } catch (error) {
    notify(`体检失败：${error.message || '请求失败'}`, 'error')
  } finally {
    checkupLoadingId.value = null
  }
}

async function fetchModels() {
  if (probing.value || saving.value || batchTesting.value || hasModelTesting.value) return
  if (!validateHeaders('探测模型')) return
  if (!form.value.base_url || !form.value.key) {
    editorError.value = '请先填写 Base URL 和 API Key'
    return
  }
  editorError.value = ''
  probing.value = true
  try {
    const data = await api.post('/channels/probe-models', {
      type: form.value.type,
      base_url: form.value.base_url,
      key: form.value.key,
      header_override: form.value.header_override || '',
    })
    const fetched = data.models || []
    const existing = new Set(models.value.map((model) => model.name))
    let added = 0
    fetched.forEach((name) => {
      if (!existing.has(name)) {
        models.value.push({ name, enabled: true, protocol: '', upstream: '', input: 0, output: 0 })
        existing.add(name)
        added += 1
      }
    })
    notify(`探测到 ${fetched.length} 个模型，新增 ${added} 个`, 'success')
  } catch (error) {
    editorError.value = `模型探测失败：${error.message || '网络错误'}`
    notify(editorError.value, 'error')
  } finally {
    probing.value = false
  }
}

function cleanPayload() {
  const cleanModels = models.value.filter((model) => model.name.trim()).map((model) => ({
    name: model.name.trim(),
    enabled: Boolean(model.enabled),
    protocol: model.protocol || '',
    upstream: model.upstream || '',
    input: Number(model.input) || 0,
    output: Number(model.output) || 0,
  }))
  const cleanRules = rules.value.filter((rule) => rule.pattern.trim() && rule.protocol)
  return {
    ...form.value,
    name: form.value.name.trim(),
    base_url: form.value.base_url.trim(),
    key: String(form.value.key || '').trim(),
    group: form.value.group.trim() || 'default',
    weight: Math.max(1, Number(form.value.weight) || 1),
    model_configs: JSON.stringify(cleanModels),
    protocol_rules: JSON.stringify(cleanRules),
    models: cleanModels.filter((model) => model.enabled).map((model) => model.name).join(','),
  }
}

async function save() {
  if (saving.value || probing.value || batchTesting.value || hasModelTesting.value) return
  if (!validateHeaders('保存')) return
  if (!canSave.value) {
    editorError.value = '请填写渠道名称、Base URL、API Key，并至少启用一个模型'
    return
  }
  editorError.value = ''
  saving.value = true
  try {
    const payload = cleanPayload()
    if (form.value.id) {
      await api.put(`/channels/${form.value.id}`, payload)
      notify('渠道已更新', 'success')
    } else {
      await api.post('/channels', payload)
      notify('渠道已创建', 'success')
    }
    showEditor.value = false
    await load()
  } catch (error) {
    editorError.value = error.message || '保存失败'
    notify(editorError.value, 'error')
  } finally {
    saving.value = false
  }
}

function toggleExpanded(channelId) {
  const next = new Set(expandedIds.value)
  if (next.has(channelId)) next.delete(channelId)
  else next.add(channelId)
  expandedIds.value = next
}

function toggleSelected(channelId) {
  const next = new Set(selectedIds.value)
  if (next.has(channelId)) next.delete(channelId)
  else next.add(channelId)
  selectedIds.value = next
}

function toggleSelectAll() {
  selectedIds.value = allSelected.value ? new Set() : new Set(channels.value.map((channel) => channel.id))
}

async function bulkDeleteChannels() {
  const ids = [...selectedIds.value]
  if (!ids.length || bulkDeleting.value) return
  if (!confirm(`确认删除选中的 ${ids.length} 个渠道？\n\n渠道及其模型路由将一并删除，此操作不可撤销。`)) return
  bulkDeleting.value = true
  try {
    await api.post('/channels/bulk-delete', { ids })
    selectedIds.value = new Set()
    notify(`已删除 ${ids.length} 个渠道`, 'success')
    await load()
  } catch (error) {
    notify(`批量删除失败：${error.message}`, 'error')
  } finally {
    bulkDeleting.value = false
  }
}

function syncChannelModelFields(channel) {
  channel.model_configs = JSON.stringify(channel._models || [])
  channel.models = (channel._models || []).filter((item) => item.enabled).map((item) => item.name).join(',')
}

async function toggleChannelModel(channel, item) {
  const key = `${channel.id}:${item.name}`
  if (modelMutating.value.has(key)) return
  const previous = item.enabled
  item.enabled = !previous
  updateSet(modelMutating, key, true)
  try {
    await api.patch(`/channels/${channel.id}/models`, { models: [item.name], enabled: item.enabled })
    syncChannelModelFields(channel)
    notify(`模型 ${item.name} 已${item.enabled ? '启用' : '停用'}`, 'success')
  } catch (error) {
    item.enabled = previous
    notify(`模型状态切换失败：${error.message}`, 'error')
  } finally {
    updateSet(modelMutating, key, false)
  }
}

async function removeChannelModel(channel, item) {
  const key = `${channel.id}:${item.name}`
  if (modelMutating.value.has(key)) return
  if (!confirm(`确认从「${channel.name}」删除模型 ${item.name}？`)) return
  updateSet(modelMutating, key, true)
  try {
    await api.delete(`/channels/${channel.id}/models`, { data: { models: [item.name] } })
    channel._models = channel._models.filter((model) => model.name !== item.name)
    syncChannelModelFields(channel)
    notify(`模型 ${item.name} 已删除`, 'success')
  } catch (error) {
    notify(`模型删除失败：${error.message}`, 'error')
  } finally {
    updateSet(modelMutating, key, false)
  }
}

async function toggleChannel(channel) {
  if (togglingIds.value.has(channel.id)) return
  const previous = channel.status
  const nextStatus = previous === 1 ? 2 : 1
  updateSet(togglingIds, channel.id, true)
  channel.status = nextStatus
  try {
    await api.patch(`/channels/${channel.id}/status`, { enabled: nextStatus === 1 })
    notify(`「${channel.name}」已${nextStatus === 1 ? '启用' : '停用'}`, 'success')
  } catch (error) {
    channel.status = previous
    notify(`状态切换失败：${error.message}`, 'error')
  } finally {
    updateSet(togglingIds, channel.id, false)
  }
}

async function removeChannel(channel) {
  if (deletingIds.value.has(channel.id)) return
  if (!confirm(`确认删除渠道「${channel.name}」？\n\n此操作不可撤销。`)) return
  updateSet(deletingIds, channel.id, true)
  try {
    await api.delete(`/channels/${channel.id}`)
    notify('渠道已删除', 'success')
    await load()
  } catch (error) {
    notify(`删除失败：${error.message}`, 'error')
  } finally {
    updateSet(deletingIds, channel.id, false)
  }
}

async function resetBreaker(channel) {
  if (resettingIds.value.has(channel.id)) return
  updateSet(resettingIds, channel.id, true)
  try {
    await api.post(`/channels/${channel.id}/health/reset`)
    channel.cooldown_until = 0
    notify(`「${channel.name}」已解除熔断`, 'success')
  } catch (error) {
    notify(`熔断复归失败：${error.message}`, 'error')
  } finally {
    updateSet(resettingIds, channel.id, false)
  }
}

function onDragStart(index, event) {
  if (reordering.value) {
    event.preventDefault()
    return
  }
  dragIndex.value = index
  dropIndex.value = index
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = 'move'
    event.dataTransfer.setData('text/plain', String(index))
  }
}

function onDragOver(index) {
  if (!reordering.value) dropIndex.value = index
}

function onDrop(index) {
  if (reordering.value) return
  const from = dragIndex.value
  if (from === null || from === index) {
    onDragEnd()
    return
  }
  const previous = channels.value.slice()
  const reordered = channels.value.slice()
  const [moved] = reordered.splice(from, 1)
  reordered.splice(index, 0, moved)
  channels.value = reordered
  onDragEnd()
  persistOrder(previous)
}

function onDragEnd() {
  dragIndex.value = null
  dropIndex.value = null
}

async function persistOrder(previous) {
  if (reordering.value) return
  reordering.value = true
  try {
    const ids = channels.value.map((channel) => channel.id)
    await api.post('/channels/reorder', { ids })
    const top = channels.value.length - 1
    channels.value.forEach((channel, index) => {
      channel.priority = top - index
    })
    notify('渠道顺序已更新', 'success')
  } catch (error) {
    channels.value = previous
    notify(`排序保存失败，已回滚：${error.message}`, 'error')
  } finally {
    reordering.value = false
  }
}

onMounted(() => {
  loadMeta()
  load()
})
</script>

<template>
  <div class="space-y-5">
    <header class="page-header">
      <div class="min-w-0">
        <div class="eyebrow">Upstream routing</div>
        <h1 class="page-title">上游渠道</h1>
        <p class="page-description">维护连接、模型与请求复写规则，并按优先级编排故障转移路径。</p>
      </div>
      <div class="page-actions">
        <button type="button" class="btn" :disabled="loading" aria-label="刷新渠道列表" @click="load">
          {{ loading ? '刷新中' : '刷新' }}
        </button>
        <button type="button" class="btn btn-primary" :disabled="metadataLoading" aria-label="新建渠道" @click="openCreate">
          ＋ 新建渠道
        </button>
      </div>
    </header>

    <section class="sheet min-w-0" aria-label="渠道列表">
      <div class="sheet-head">
        <div class="min-w-0">
          <span class="dim-title">渠道列表</span>
          <div class="mt-1 text-[12px] text-soft">按当前顺序分配优先级，可拖动调整。</div>
        </div>
        <div class="flex flex-wrap items-center justify-end gap-2">
          <button v-if="selectedIds.size" type="button" class="btn btn-danger btn-sm" :disabled="bulkDeleting" @click="bulkDeleteChannels">{{ bulkDeleting ? '删除中' : `删除已选 ${selectedIds.size} 项` }}</button>
          <span v-if="reordering" class="chip chip-test" role="status">正在保存顺序</span>
          <span v-else class="chip">{{ channels.length }} 个渠道</span>
        </div>
      </div>

      <PageState
        :loading="loading"
        :error="loadError"
        :empty="!channels.length"
        empty-text="暂无渠道"
        @retry="load"
      >
        <div class="hidden md:block">
          <table class="table-eng w-full table-fixed" aria-label="渠道列表">
            <thead>
              <tr>
                <th class="w-24"><label class="flex items-center gap-2"><input type="checkbox" :checked="allSelected" aria-label="选择全部渠道" @change="toggleSelectAll" />顺序</label></th>
                <th class="w-28">状态</th>
                <th class="w-[22%]">渠道</th>
                <th>连接地址</th>
                <th class="w-20 text-right">模型</th>
                <th class="w-32">调用健康</th>
                <th class="w-20 text-right">权重</th>
                <th class="w-56 text-right">操作</th>
              </tr>
            </thead>
            <tbody>
              <template v-for="(channel, index) in sortedChannels" :key="channel.id">
              <tr
                :class="[
                  dragIndex === index ? 'opacity-40' : '',
                  dropIndex === index && dragIndex !== null && dragIndex !== index ? 'outline outline-2 outline-blue outline-offset-[-2px]' : '',
                  channel.status !== 1 ? 'text-soft' : '',
                ]"
                :draggable="!reordering"
                @dragstart="onDragStart(index, $event)"
                @dragover.prevent="onDragOver(index)"
                @drop.prevent="onDrop(index)"
                @dragend="onDragEnd"
              >
                <td>
                  <div class="flex items-center gap-2">
                    <input type="checkbox" :checked="selectedIds.has(channel.id)" :aria-label="`选择渠道 ${channel.name}`" @change="toggleSelected(channel.id)" />
                    <button type="button" class="btn btn-sm px-1" :aria-expanded="expandedIds.has(channel.id)" :aria-label="`${expandedIds.has(channel.id) ? '收起' : '展开'} ${channel.name} 的模型`" @click="toggleExpanded(channel.id)">{{ expandedIds.has(channel.id) ? '−' : '+' }}</button>
                    <button
                      type="button"
                      class="btn btn-sm cursor-grab px-1 active:cursor-grabbing"
                      :disabled="reordering"
                      :aria-label="`拖动调整 ${channel.name} 的优先级`"
                    >
                      <span aria-hidden="true">⠿</span>
                    </button>
                    <span class="font-mono text-[13px] font-medium">{{ String(index + 1).padStart(2, '0') }}</span>
                  </div>
                </td>
                <td>
                  <div class="flex items-center gap-2">
                    <button type="button" class="channel-switch shrink-0" :class="{ 'channel-switch-on': channel.status === 1 }" :disabled="togglingIds.has(channel.id)" :aria-pressed="channel.status === 1" :aria-label="`${channel.status === 1 ? '停用' : '启用'}渠道 ${channel.name}`" @click="toggleChannel(channel)"><span aria-hidden="true"></span></button>
                    <span class="chip" :class="{
                      'chip-run': breakerState(channel) === 'run',
                      'chip-test': breakerState(channel) === 'test',
                      'chip-trip': breakerState(channel) === 'trip',
                    }">{{ breakerText(channel) }}</span>
                  </div>
                </td>
                <td>
                  <div class="truncate font-cond text-[15px] font-semibold tracking-wide text-ink" :title="channel.name">{{ channel.name }}</div>
                  <div class="mt-1 flex flex-wrap gap-1.5">
                    <span class="chip chip-blue">{{ typeName(channel.type) }}</span>
                    <span class="chip">{{ channel.group || 'default' }}</span>
                    <span class="chip">ID {{ channel.id }}</span>
                  </div>
                </td>
                <td>
                  <code class="block break-all text-[11px] text-ink">{{ channel.base_url || '使用默认地址' }}</code>
                  <div class="mt-1 truncate font-mono text-[11px] text-faint">Key：{{ maskedKey(channel.key) }}</div>
                </td>
                <td class="num">{{ modelCount(channel) }}</td>
                <td><span class="chip" :class="healthClass(channelHealth(channel))" :title="healthTitle(channelHealth(channel))">{{ healthText(channelHealth(channel)) }}</span></td>
                <td class="num">×{{ channel.weight }}</td>
                <td>
                  <div class="flex items-center justify-end gap-1.5">
                    <button type="button" class="btn btn-sm" :disabled="checkupLoadingId !== null" :aria-label="`测试 ${channel.name}`" @click="checkupChannel(channel)">
                      {{ checkupLoadingId === channel.id ? '测试中' : '测试' }}
                    </button>
                    <button type="button" class="btn btn-sm" :aria-label="`编辑渠道 ${channel.name}`" @click="openEdit(channel)">编辑</button>
                    <ActionMenu>
                      <button role="menuitem" type="button" :disabled="!channel.key" @click.stop="copyKey(channel)">复制 Key</button>
                      <button role="menuitem" type="button" :disabled="togglingIds.has(channel.id)" @click.stop="toggleChannel(channel)">{{ togglingIds.has(channel.id) ? '切换中' : channel.status === 1 ? '停用' : '启用' }}</button>
                      <button role="menuitem" type="button" :disabled="resettingIds.has(channel.id)" @click.stop="resetBreaker(channel)">{{ resettingIds.has(channel.id) ? '处理中' : '解除熔断' }}</button>
                      <button role="menuitem" type="button" class="text-trip" :disabled="deletingIds.has(channel.id)" @click.stop="removeChannel(channel)">{{ deletingIds.has(channel.id) ? '删除中' : '删除' }}</button>
                    </ActionMenu>
                  </div>
                </td>
              </tr>
              <tr v-if="expandedIds.has(channel.id)">
                <td colspan="8" class="!p-0">
                  <div class="border-y border-line bg-canvas/70 px-5 py-4">
                    <div class="mb-3 flex items-center justify-between gap-3"><span class="dim-title">渠道模型</span><span class="text-xs text-soft">{{ channel._models.length }} 个配置</span></div>
                    <div v-if="channel._models.length" class="grid gap-2 lg:grid-cols-2">
                      <article v-for="item in channel._models" :key="item.name" class="flex min-w-0 items-center gap-3 rounded-lg border border-line bg-white p-3">
                        <button type="button" class="channel-switch shrink-0" :class="{ 'channel-switch-on': item.enabled }" :disabled="modelMutating.has(`${channel.id}:${item.name}`)" :aria-pressed="item.enabled" :aria-label="`${item.enabled ? '停用' : '启用'}模型 ${item.name}`" @click="toggleChannelModel(channel, item)"><span aria-hidden="true"></span></button>
                        <div class="min-w-0 flex-1"><code class="block truncate text-xs text-ink" :title="item.name">{{ item.name }}</code><div class="mt-1 truncate text-[11px] text-soft">{{ item.protocol || '继承协议' }} · → {{ item.upstream || item.name }}</div></div>
                        <span class="chip shrink-0" :class="healthClass(modelHealth(channel, item))" :title="healthTitle(modelHealth(channel, item))">{{ healthText(modelHealth(channel, item)) }}</span>
                        <span class="chip shrink-0" :class="item.enabled && channel.status === 1 ? 'chip-run' : ''">{{ item.enabled ? '启用' : '停用' }}</span>
                        <button type="button" class="btn btn-danger btn-sm shrink-0" :disabled="modelMutating.has(`${channel.id}:${item.name}`)" @click="removeChannelModel(channel, item)">删除</button>
                      </article>
                    </div>
                    <div v-else class="rounded-lg border border-dashed border-line py-6 text-center text-sm text-soft">此渠道尚未配置模型</div>
                  </div>
                </td>
              </tr>
              </template>
            </tbody>
          </table>
        </div>

        <div class="grid gap-3 p-3 md:hidden">
          <article
            v-for="(channel, index) in sortedChannels"
            :key="channel.id"
            class="min-w-0 border border-line bg-white p-3"
            :draggable="!reordering"
            @dragstart="onDragStart(index, $event)"
            @dragover.prevent="onDragOver(index)"
            @drop.prevent="onDrop(index)"
            @dragend="onDragEnd"
          >
            <div class="flex items-start justify-between gap-3">
              <div class="min-w-0">
                <div class="flex flex-wrap items-center gap-2">
                  <input type="checkbox" :checked="selectedIds.has(channel.id)" :aria-label="`选择渠道 ${channel.name}`" @change="toggleSelected(channel.id)" />
                  <button type="button" class="btn btn-sm px-1" :aria-expanded="expandedIds.has(channel.id)" @click="toggleExpanded(channel.id)">{{ expandedIds.has(channel.id) ? '−' : '+' }}</button>
                  <button type="button" class="btn btn-sm cursor-grab px-1" :disabled="reordering" :aria-label="`拖动调整 ${channel.name} 的顺序`">⠿</button>
                  <span class="font-mono text-[11px] text-faint">#{{ index + 1 }}</span>
                  <span class="chip" :class="{ 'chip-run': breakerState(channel) === 'run', 'chip-test': breakerState(channel) === 'test', 'chip-trip': breakerState(channel) === 'trip' }">{{ breakerText(channel) }}</span>
                </div>
                <h2 class="mt-2 break-words font-medium text-ink">{{ channel.name }}</h2>
              </div>
              <ActionMenu>
                <button role="menuitem" type="button" :disabled="!channel.key" @click.stop="copyKey(channel)">复制 Key</button>
                <button role="menuitem" type="button" :disabled="togglingIds.has(channel.id)" @click.stop="toggleChannel(channel)">{{ channel.status === 1 ? '停用' : '启用' }}</button>
                <button role="menuitem" type="button" :disabled="resettingIds.has(channel.id)" @click.stop="resetBreaker(channel)">解除熔断</button>
                <button role="menuitem" type="button" class="text-trip" :disabled="deletingIds.has(channel.id)" @click.stop="removeChannel(channel)">删除</button>
              </ActionMenu>
            </div>
            <div class="mt-2 flex flex-wrap gap-1">
              <span class="chip chip-blue">{{ typeName(channel.type) }}</span>
              <span class="chip">{{ channel.group || 'default' }}</span>
              <span class="chip">{{ modelCount(channel) }} 个模型</span>
              <span class="chip" :class="healthClass(channelHealth(channel))" :title="healthTitle(channelHealth(channel))">健康 {{ healthText(channelHealth(channel)) }}</span>
              <span class="chip">权重 {{ channel.weight }}</span>
            </div>
            <code class="mt-3 block break-all text-[11px] text-soft">{{ channel.base_url || '使用默认地址' }}</code>
            <div class="mt-3 grid grid-cols-2 gap-2">
              <button type="button" class="btn btn-sm" :disabled="checkupLoadingId !== null" @click="checkupChannel(channel)">{{ checkupLoadingId === channel.id ? '测试中' : '测试' }}</button>
              <button type="button" class="btn btn-sm" @click="openEdit(channel)">编辑</button>
            </div>
            <div v-if="expandedIds.has(channel.id)" class="mt-3 space-y-2 border-t border-line pt-3">
              <div v-for="item in channel._models" :key="item.name" class="flex min-w-0 items-center gap-2 rounded border border-line p-2">
                <button type="button" class="channel-switch shrink-0" :class="{ 'channel-switch-on': item.enabled }" :disabled="modelMutating.has(`${channel.id}:${item.name}`)" @click="toggleChannelModel(channel, item)"><span></span></button>
                <div class="min-w-0 flex-1"><code class="block truncate text-xs">{{ item.name }}</code><span class="block truncate text-[10px] text-soft">→ {{ item.upstream || item.name }}</span></div>
                <span class="chip shrink-0" :class="healthClass(modelHealth(channel, item))" :title="healthTitle(modelHealth(channel, item))">{{ healthText(modelHealth(channel, item)) }}</span>
                <button type="button" class="btn btn-danger btn-sm" :disabled="modelMutating.has(`${channel.id}:${item.name}`)" @click="removeChannelModel(channel, item)">删除</button>
              </div>
              <div v-if="!channel._models.length" class="text-center text-xs text-soft">暂无模型</div>
            </div>
          </article>
        </div>
      </PageState>
    </section>

    <section class="grid gap-3 md:grid-cols-2" aria-label="渠道说明">
      <div class="border border-line bg-white p-3">
        <div class="eyebrow">优先级</div>
        <p class="mt-1 text-[13px] text-soft">列表越靠前优先级越高；拖动渠道后立即保存，失败时自动恢复原顺序。</p>
      </div>
      <div class="border border-line bg-white p-3">
        <div class="eyebrow">模型价格</div>
        <p class="mt-1 text-[13px] text-soft">输入价与输出价单位为 USD / 1M tokens；填写 0 时使用全局价格。</p>
      </div>
    </section>

    <Modal
      :open="showEditor"
      :title="form.id ? `编辑渠道 · ${form.name}` : '新建渠道'"
      width="max-w-6xl"
      :persistent="editorBusy"
      @close="closeEditor"
    >
      <div class="min-w-0 space-y-4">
        <div class="flex gap-1 overflow-x-auto border-b border-line" role="tablist" aria-label="渠道配置">
          <button type="button" class="tab-button" :class="{ 'tab-button-active': editorTab === 'connection' }" role="tab" :aria-selected="editorTab === 'connection'" @click="editorTab = 'connection'">连接</button>
          <button type="button" class="tab-button" :class="{ 'tab-button-active': editorTab === 'models' }" role="tab" :aria-selected="editorTab === 'models'" @click="editorTab = 'models'">模型</button>
          <button type="button" class="tab-button" :class="{ 'tab-button-active': editorTab === 'routing' }" role="tab" :aria-selected="editorTab === 'routing'" @click="editorTab = 'routing'">路由与请求头</button>
        </div>
        <section v-show="editorTab === 'connection'" class="border border-line bg-white" aria-labelledby="nameplate-heading">
          <div class="flex flex-wrap items-center justify-between gap-2 border-b border-ink bg-panel px-3 py-2">
            <div>
              <div id="nameplate-heading" class="font-cond text-sm font-semibold tracking-wide">连接设置</div>
              <div class="mt-0.5 text-[12px] text-soft">配置渠道名称、地址和访问凭据。</div>
            </div>
            <span class="chip" :class="form.status === 1 ? 'chip-run' : ''">{{ form.status === 1 ? '启用' : '停用' }}</span>
          </div>
          <div class="grid gap-3 p-3 md:grid-cols-2 xl:grid-cols-4">
            <div class="xl:col-span-2">
              <label class="field-label" for="channel-name">渠道名称 *</label>
              <input id="channel-name" v-model="form.name" class="input" placeholder="例：OpenAI 主账号" autocomplete="off" data-autofocus />
            </div>
            <div>
              <label class="field-label" for="channel-type">默认协议 *</label>
              <select id="channel-type" v-model.number="form.type" class="input" @change="onTypeChange">
                <option v-for="type in channelTypes" :key="type.value" :value="type.value">{{ type.name }}</option>
              </select>
            </div>
            <div>
              <label class="field-label" for="channel-group">分组</label>
              <input id="channel-group" v-model="form.group" class="input input-mono" placeholder="default" autocomplete="off" />
            </div>
            <div class="md:col-span-2">
              <label class="field-label" for="channel-url">Base URL *</label>
              <input id="channel-url" v-model="form.base_url" class="input input-mono" placeholder="https://api.openai.com" autocomplete="off" />
            </div>
            <div class="md:col-span-2">
              <label class="field-label" for="channel-key">API Key *</label>
              <div class="flex gap-2">
                <input
                  id="channel-key"
                  v-model="form.key"
                  type="text"
                  class="input input-mono"
                  placeholder="upstream-key"
                  name="apirelay-upstream-key"
                  autocomplete="off"
                  autocapitalize="off"
                  autocorrect="off"
                  spellcheck="false"
                  data-1p-ignore
                  data-lpignore="true"
                  data-form-type="other"
                />
                <button type="button" class="btn shrink-0" :disabled="!form.key" aria-label="复制编辑器中的 API Key" @click="copyKey(form)">复制</button>
              </div>
              <p class="mt-1 text-[12px] text-soft">上游凭据按现有数据结构保存；复制操作使用安全上下文与兼容降级。</p>
            </div>
            <div class="md:col-span-2 xl:col-span-4">
              <label class="field-label" for="channel-test-prompt">测试提示词覆盖</label>
              <textarea id="channel-test-prompt" v-model="form.test_prompt" class="input min-h-24 resize-y" maxlength="4000" :placeholder="`留空继承全局：${globalTestPrompt}`"></textarea>
              <p class="mt-1 text-[12px] text-soft">单测与批量体检优先使用此内容；留空时继承设置页中的全局默认。</p>
            </div>
          </div>
        </section>

        <section v-show="editorTab === 'models'" class="border border-line bg-white" aria-labelledby="circuits-heading">
          <div class="flex flex-wrap items-center justify-between gap-2 border-b border-ink bg-panel px-3 py-2">
            <div>
              <div id="circuits-heading" class="font-cond text-sm font-semibold tracking-wide">模型配置</div>
              <div class="mt-0.5 text-[12px] text-soft">维护模型名称、协议、映射和价格。</div>
            </div>
            <div class="flex flex-wrap gap-2">
              <button
                type="button"
                class="btn btn-sm"
                :disabled="editorBusy || !form.base_url || !form.key || enabledCount === 0"
                aria-label="测试全部启用模型"
                @click="testAllInModal"
              >{{ batchTesting ? `批测中 ${batchDone}/${batchTotal}` : '批量测试' }}</button>
              <button
                type="button"
                class="btn btn-sm"
                :disabled="editorBusy || !form.base_url || !form.key"
                aria-label="从上游探测模型列表"
                @click="fetchModels"
              >{{ probing ? '探测中' : '模型探测' }}</button>
            </div>
            <p class="w-full text-right text-[11px] text-soft">测试与探测将携带 {{ customHeaderCount }} 个自定义请求头</p>
          </div>

          <div class="p-3">
            <div v-if="batchTesting || batchSummary" class="mb-3 flex flex-wrap items-center gap-2" aria-live="polite">
              <span v-if="batchTesting" class="chip chip-test">执行中 {{ batchDone }}/{{ batchTotal }}</span>
              <template v-if="batchSummary">
                <span class="chip chip-run">通过 {{ batchSummary.success }}</span>
                <span class="chip chip-trip">失败 {{ batchSummary.failed }}</span>
                <span class="chip chip-test">总计 {{ batchSummary.total }}</span>
              </template>
            </div>

            <div class="mb-3 flex flex-col gap-2 sm:flex-row">
              <input
                v-model="newModelName"
                class="input input-mono"
                placeholder="模型显示名（可使用 * 通配）"
                aria-label="新模型名称"
                @keyup.enter="addModel"
              />
              <button type="button" class="btn shrink-0" aria-label="添加模型" @click="addModel">＋ 添加模型</button>
            </div>

            <div v-if="models.length" class="hidden border border-line md:block">
              <table class="table-eng w-full table-fixed" aria-label="模型配置表">
                <thead>
                  <tr>
                    <th class="w-16">启用</th>
                    <th>模型名称</th>
                    <th class="w-36">协议</th>
                    <th>上游映射</th>
                    <th class="w-24 text-right">输入价</th>
                    <th class="w-24 text-right">输出价</th>
                    <th class="w-24 text-right">测试</th>
                    <th class="w-20 text-right">删除</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="(model, index) in models" :key="index">
                    <td>
                      <button
                        type="button"
                        class="channel-switch"
                        :class="{ 'channel-switch-on': model.enabled }"
                        :aria-pressed="model.enabled"
                        :aria-label="`${model.enabled ? '停用' : '启用'}模型 ${model.name || index + 1}`"
                        @click="model.enabled = !model.enabled"
                      ><span aria-hidden="true"></span></button>
                    </td>
                    <td><input v-model="model.name" class="input input-mono py-1 text-[12px]" placeholder="显示名" :aria-label="`第 ${index + 1} 个模型显示名`" /></td>
                    <td>
                      <select v-model="model.protocol" class="input py-1 text-[12px]" :aria-label="`${model.name || index + 1} 的协议覆盖`">
                        <option value="">继承规则</option>
                        <option v-for="protocol in protocols" :key="protocol.value" :value="protocol.value">{{ protocol.name }}</option>
                      </select>
                    </td>
                    <td><input v-model="model.upstream" class="input input-mono py-1 text-[12px]" placeholder="留空则同显示名" :aria-label="`${model.name || index + 1} 的上游模型映射`" /></td>
                    <td><input v-model.number="model.input" type="number" step="0.01" min="0" class="input py-1 text-right font-mono text-[12px]" placeholder="0" :aria-label="`${model.name || index + 1} 的输入价格`" /></td>
                    <td><input v-model.number="model.output" type="number" step="0.01" min="0" class="input py-1 text-right font-mono text-[12px]" placeholder="0" :aria-label="`${model.name || index + 1} 的输出价格`" /></td>
                    <td class="text-right">
                      <button
                        type="button"
                        class="btn btn-sm"
                        :disabled="Boolean(testing[model.name]) || batchTesting || saving || !model.name.trim()"
                        :aria-label="`测试模型 ${model.name || index + 1}`"
                        @click="testModel(model)"
                      >{{ testing[model.name] ? '测试中' : '单测' }}</button>
                    </td>
                    <td class="text-right">
                      <button
                        type="button"
                        class="btn btn-danger btn-sm"
                        :disabled="Boolean(testing[model.name]) || batchTesting"
                        :aria-label="`删除模型 ${model.name || index + 1}`"
                        @click="removeModel(index, model)"
                      >删除</button>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
            <div v-if="models.length" class="grid gap-3 md:hidden">
              <article v-for="(model, index) in models" :key="index" class="space-y-3 border border-line p-3">
                <div class="flex items-center justify-between gap-2">
                  <span class="font-medium">模型 {{ index + 1 }}</span>
                  <button type="button" class="channel-switch" :class="{ 'channel-switch-on': model.enabled }" :aria-pressed="model.enabled" @click="model.enabled = !model.enabled"><span></span></button>
                </div>
                <input v-model="model.name" class="input input-mono" placeholder="模型名称" />
                <select v-model="model.protocol" class="input"><option value="">继承规则</option><option v-for="protocol in protocols" :key="protocol.value" :value="protocol.value">{{ protocol.name }}</option></select>
                <input v-model="model.upstream" class="input input-mono" placeholder="上游映射，留空则同模型名称" />
                <div class="grid grid-cols-2 gap-2"><input v-model.number="model.input" type="number" step="0.01" min="0" class="input" placeholder="输入价" /><input v-model.number="model.output" type="number" step="0.01" min="0" class="input" placeholder="输出价" /></div>
                <div class="grid grid-cols-2 gap-2"><button type="button" class="btn btn-sm" :disabled="Boolean(testing[model.name]) || batchTesting || saving || !model.name.trim() || !headerValidation.valid" @click="testModel(model)">{{ testing[model.name] ? '测试中' : '单测' }}</button><button type="button" class="btn btn-danger btn-sm" :disabled="Boolean(testing[model.name]) || batchTesting" @click="removeModel(index, model)">删除</button></div>
              </article>
            </div>
            <div v-if="!models.length" class="my-5 border border-dashed border-line p-6 text-center text-[13px] text-soft">尚未添加模型</div>
            <p class="mt-2 text-[12px] text-soft">已启用 {{ enabledCount }} 个；协议按模型显式配置、正则规则、渠道默认协议的顺序解析。</p>

            <div v-if="testRecordRows.length" class="mt-4 border border-line">
              <table class="table-eng w-full table-fixed" aria-label="模型测试记录表">
                <thead>
                  <tr>
                    <th>模型</th>
                    <th class="w-24">结果</th>
                    <th class="w-28">协议</th>
                    <th>上游模型</th>
                    <th class="w-24 text-right">延迟</th>
                    <th>说明</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="row in testRecordRows" :key="row.model.name">
                    <td><code class="text-[12px]">{{ row.model.name }}</code></td>
                    <td>
                      <span v-if="testing[row.model.name] || row.result?.pending" class="chip chip-test">测试中</span>
                      <span v-else-if="row.result?.success" class="chip chip-run">通过</span>
                      <span v-else class="chip chip-trip">失败</span>
                    </td>
                    <td><code class="text-[12px]">{{ row.result?.protocol || '—' }}</code></td>
                    <td><code class="break-all text-[12px]">{{ row.result?.upstream || row.model.upstream || row.model.name }}</code></td>
                    <td class="num">{{ row.result?.latency_ms ? `${row.result.latency_ms} ms` : '—' }}</td>
                    <td class="max-w-md break-words text-[12px] text-soft">{{ row.result?.success ? (row.result.reply || '连通正常') : (row.result?.error || '等待试验结果') }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </section>

        <div v-show="editorTab === 'routing'" class="grid gap-4 lg:grid-cols-2">
          <section class="border border-ink bg-white" aria-labelledby="rules-heading">
            <div class="border-b border-ink bg-panel px-3 py-2">
              <div id="rules-heading" class="font-cond text-sm font-semibold tracking-wide">协议路由规则</div>
              <div class="mt-0.5 text-[12px] text-soft">按模型名称匹配目标协议。</div>
            </div>
            <div class="space-y-2 p-3">
              <p class="text-[12px] text-soft">按模型名称正则匹配；优先级低于模型中的显式协议配置。</p>
              <div v-for="(rule, index) in rules" :key="index" class="grid min-w-0 grid-cols-1 gap-2 sm:grid-cols-[minmax(0,1fr)_130px_auto]">
                <input v-model="rule.pattern" class="input input-mono text-[12px]" placeholder="^claude" :aria-label="`第 ${index + 1} 条协议规则正则`" />
                <select v-model="rule.protocol" class="input text-[12px]" :aria-label="`第 ${index + 1} 条协议规则目标协议`">
                  <option v-for="protocol in protocols" :key="protocol.value" :value="protocol.value">{{ protocol.name }}</option>
                </select>
                <button type="button" class="btn btn-danger btn-sm" :aria-label="`删除第 ${index + 1} 条协议规则`" @click="rules.splice(index, 1)">删除</button>
              </div>
              <button type="button" class="btn btn-sm" aria-label="添加协议规则" @click="rules.push({ pattern: '', protocol: 'anthropic' })">＋ 添加规则</button>
            </div>
          </section>

          <section class="border border-ink bg-white" aria-labelledby="advanced-heading">
            <div class="border-b border-ink bg-panel px-3 py-2">
              <div id="advanced-heading" class="font-cond text-sm font-semibold tracking-wide">权重与请求复写</div>
              <div class="mt-0.5 text-[12px] text-soft">设置负载权重，并在协议转换后复写发往上游的请求头与请求体。</div>
            </div>
            <div class="grid gap-3 p-3">
              <div>
                <label class="field-label" for="channel-weight">渠道权重</label>
                <input id="channel-weight" v-model.number="form.weight" type="number" min="1" class="input input-mono" placeholder="1" />
                <p class="mt-1 text-[12px] text-soft">同优先级条件下的负载分配比例；优先级可在渠道列表中拖动调整。</p>
              </div>
              <HeaderOverrideEditor
                v-model="form.header_override"
                :disabled="editorBusy"
                @validation="headerValidation = $event"
              />
              <BodyOverrideEditor
                v-model="form.body_override"
                :disabled="editorBusy"
                @validation="bodyValidation = $event"
              />
            </div>
          </section>
        </div>

        <div v-if="editorError" class="border border-trip bg-trip-wash px-3 py-2 text-[13px] text-trip" role="alert">{{ editorError }}</div>
      </div>

      <template #footer>
        <div class="flex w-full flex-wrap items-center justify-between gap-2">
          <span class="font-mono text-2xs text-faint">{{ form.id ? `CHANNEL ID ${form.id}` : 'UNSAVED CHANNEL' }}</span>
          <div class="flex gap-2">
            <button type="button" class="btn" :disabled="editorBusy" aria-label="取消渠道编辑" @click="closeEditor">取消</button>
            <button
              type="button"
              class="btn btn-primary"
              :disabled="editorBusy || !canSave"
              aria-label="保存渠道配置"
              @click="save"
            >{{ saving ? '保存中' : '保存' }}</button>
          </div>
        </div>
      </template>
    </Modal>

    <Modal
      :open="showCheckup"
      :title="`已保存渠道体检 · ${checkupChannelName}`"
      width="max-w-4xl"
      @close="showCheckup = false"
    >
      <div class="space-y-3">
        <div v-if="checkupSummary" class="flex flex-wrap items-center gap-2">
          <span class="chip chip-run">通过 {{ checkupSummary.success }}</span>
          <span class="chip chip-trip">失败 {{ checkupSummary.failed }}</span>
          <span class="chip chip-test">总计 {{ checkupSummary.total }}</span>
          <span class="chip">合格率 {{ checkupRate }}%</span>
        </div>

        <div v-if="checkupResults.length" class="space-y-2">
          <article v-for="(result, index) in checkupResults" :key="`${result.model}-${index}`" class="rounded-lg border border-line p-3">
            <div class="flex flex-wrap items-center justify-between gap-2">
              <code class="break-all text-[12px]">{{ result.model }}</code>
              <span class="chip" :class="result.success ? 'chip-run' : 'chip-trip'">{{ result.success ? '通过' : '失败' }}</span>
            </div>
            <div class="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-soft">
              <span>协议：<code>{{ result.protocol || '—' }}</code></span>
              <span>上游：<code>{{ result.upstream || result.model }}</code></span>
              <span>延迟：<code>{{ result.success && result.latency_ms ? `${result.latency_ms} ms` : '—' }}</code></span>
            </div>
            <p class="mt-2 break-words text-xs leading-5 text-soft">{{ result.success ? (result.reply || '连通正常') : (result.error || '未返回错误说明') }}</p>
          </article>
        </div>
        <div v-else class="rounded-lg border border-dashed border-line px-4 py-8 text-center text-sm text-soft">无体检记录</div>
      </div>

      <template #footer>
        <button type="button" class="btn" aria-label="关闭渠道体检记录" @click="showCheckup = false">关闭记录</button>
      </template>
    </Modal>
  </div>
</template>
