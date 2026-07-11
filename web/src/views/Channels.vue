<script setup>
import { computed, getCurrentInstance, onMounted, ref } from 'vue'
import api, { copyText } from '../api'
import { DEFAULT_HEALTH_CONFIG, hasHealth, healthTotal, healthText, healthTitle, healthClass as healthClassBy } from '../health'
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
const revealKey = ref(false)
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

const selectedIds = ref(new Set())
const globalTestPrompt = ref("Say 'hi' in one word.")
const healthConfig = ref({ ...DEFAULT_HEALTH_CONFIG })
const bulkDeleting = ref(false)

const togglingIds = ref(new Set())
const deletingIds = ref(new Set())
const resettingIds = ref(new Set())
const dragIndex = ref(null)
const dropIndex = ref(null)
const reordering = ref(false)
const channelQuery = ref('')
const statusFilter = ref('all')

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

const sortedChannels = computed(() => {
  const query = channelQuery.value.trim().toLowerCase()
  return channels.value.filter((channel) => {
    const state = breakerState(channel)
    if (statusFilter.value !== 'all' && state !== statusFilter.value) return false
    if (!query) return true
    const haystack = [channel.name, channel.group, channel.base_url, typeName(channel.type), ...(channel._models || []).map((item) => item.name)]
      .filter(Boolean)
      .join(' ')
      .toLowerCase()
    return haystack.includes(query)
  })
})
const canReorder = computed(() => statusFilter.value === 'all' && !channelQuery.value.trim())
const channelSummary = computed(() => channels.value.reduce((summary, channel) => {
  const state = breakerState(channel)
  summary.total += 1
  summary[state] = (summary[state] || 0) + 1
  return summary
}, { total: 0, run: 0, trip: 0, off: 0, test: 0 }))
const routeSegments = computed(() => [
  { key: 'run', label: '运行', count: channelSummary.value.run, tone: 'run' },
  { key: 'test', label: '检查', count: channelSummary.value.test, tone: 'test' },
  { key: 'trip', label: '熔断', count: channelSummary.value.trip, tone: 'trip' },
  { key: 'off', label: '停用', count: channelSummary.value.off, tone: 'off' },
].map((item) => ({
  ...item,
  percent: channelSummary.value.total ? Math.round((item.count / channelSummary.value.total) * 100) : 0,
})))
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
const saveHint = computed(() => {
  if (!form.value.name.trim()) return '填写渠道名称后可保存'
  if (!form.value.base_url.trim()) return '填写 Base URL 后可保存'
  if (!String(form.value.key || '').trim()) return '填写 API Key 后可保存'
  if (!enabledCount.value) return '至少启用一个模型后可保存'
  if (!headerValidation.value.valid) return '修正请求头配置后可保存'
  if (!bodyValidation.value.valid) return '修正请求体配置后可保存'
  return '配置完整，可以保存'
})
const editorSteps = computed(() => ({
  connection: Boolean(form.value.name.trim() && form.value.base_url.trim() && String(form.value.key || '').trim()),
  models: enabledCount.value > 0,
  routing: headerValidation.value.valid && bodyValidation.value.valid,
}))
const activeEditor = computed(() => ({
  connection: { index: '01', title: '连接与凭据', note: '定义渠道身份及上游接入方式。' },
  models: { index: '02', title: '模型工作台', note: '维护模型、映射、协议和计价。' },
  routing: { index: '03', title: '流量策略与请求复写', note: '控制权重、协议匹配和上游请求内容。' },
}[editorTab.value]))
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

function displayEndpoint(value) {
  const raw = String(value || '').trim()
  if (!raw) return '未配置地址'
  try {
    const url = new URL(raw)
    return `${url.host}${url.pathname === '/' ? '' : url.pathname}`
  } catch {
    return raw.replace(/^https?:\/\//, '')
  }
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

// 依据当前设置阈值返回健康 chip class。
function healthClass(health) {
  return healthClassBy(health, healthConfig.value)
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
    const [types, protocolList, promptData, healthData] = await Promise.all([
      api.get('/channel-types'),
      api.get('/protocols'),
      api.get('/settings/test-prompt'),
      api.get('/settings/model-health'),
    ])
    channelTypes.value = types || []
    protocols.value = protocolList || []
    globalTestPrompt.value = promptData?.prompt || globalTestPrompt.value
    healthConfig.value = { ...healthConfig.value, ...(healthData || {}) }
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
  revealKey.value = false
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

function toggleSelected(channelId) {
  const next = new Set(selectedIds.value)
  if (next.has(channelId)) next.delete(channelId)
  else next.add(channelId)
  selectedIds.value = next
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
  const wasTripped = breakerState(channel) === 'trip'
  updateSet(resettingIds, channel.id, true)
  try {
    await api.post(`/channels/${channel.id}/health/reset`)
    channel.cooldown_until = 0
    notify(wasTripped ? `「${channel.name}」已解除熔断` : `「${channel.name}」健康状态已重置`, 'success')
  } catch (error) {
    notify(`健康状态重置失败：${error.message}`, 'error')
  } finally {
    updateSet(resettingIds, channel.id, false)
  }
}

function onDragStart(index, event) {
  if (reordering.value || !canReorder.value) {
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
          新建渠道
        </button>
      </div>
    </header>

    <section class="sheet channel-console min-w-0 overflow-hidden" aria-label="渠道列表">
      <div class="border-b border-line bg-white px-4 py-4 sm:px-5">
        <div class="flex flex-col gap-4 xl:flex-row xl:items-end xl:justify-between">
          <div class="min-w-0 flex-1">
            <div class="flex flex-wrap items-baseline gap-x-3 gap-y-1">
              <span class="dim-title">路由运行台</span>
              <span class="font-mono text-[10px] uppercase tracking-[0.14em] text-faint">{{ channelSummary.total }} feeders online</span>
            </div>
            <div class="mt-1 text-[12px] text-soft">点击母线区段快速筛选；列表顺序即故障转移优先级。</div>
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <button v-if="selectedIds.size" type="button" class="btn btn-danger btn-sm" :disabled="bulkDeleting" @click="bulkDeleteChannels">{{ bulkDeleting ? '删除中' : `删除已选 ${selectedIds.size} 项` }}</button>
            <span v-if="reordering" class="chip chip-test" role="status">正在同步优先级</span>
            <span v-else class="chip">显示 {{ sortedChannels.length }} / {{ channels.length }}</span>
          </div>
        </div>

        <div class="route-bus mt-4" aria-label="路由状态母线">
          <button
            v-for="segment in routeSegments"
            :key="segment.key"
            type="button"
            class="route-bus-segment"
            :class="[`route-bus-${segment.tone}`, statusFilter === segment.key ? 'route-bus-active' : '']"
            :style="{ '--segment-grow': Math.max(segment.count, 1) }"
            :aria-pressed="statusFilter === segment.key"
            @click="statusFilter = statusFilter === segment.key ? 'all' : segment.key"
          >
            <span class="route-bus-line" aria-hidden="true"></span>
            <span class="route-bus-copy"><b>{{ segment.count }}</b><span>{{ segment.label }} · {{ segment.percent }}%</span></span>
          </button>
        </div>

        <div class="mt-4 grid gap-3 lg:grid-cols-[minmax(280px,1fr)_auto] lg:items-center">
          <label class="relative block">
            <span class="pointer-events-none absolute inset-y-0 left-3 flex items-center text-faint" aria-hidden="true">⌕</span>
            <span class="sr-only">搜索渠道</span>
            <input v-model="channelQuery" class="input input-mono pl-9" type="search" placeholder="搜索渠道、分组、地址或模型" />
          </label>
          <div class="flex flex-wrap gap-1.5" aria-label="渠道状态筛选">
            <button type="button" class="chip" :class="statusFilter === 'all' ? 'chip-blue' : ''" :aria-pressed="statusFilter === 'all'" @click="statusFilter = 'all'">全部 {{ channelSummary.total }}</button>
            <button v-for="segment in routeSegments" :key="`filter-${segment.key}`" type="button" class="chip" :class="statusFilter === segment.key ? `chip-${segment.tone === 'off' ? 'test' : segment.tone}` : ''" :aria-pressed="statusFilter === segment.key" @click="statusFilter = segment.key">{{ segment.label }} {{ segment.count }}</button>
          </div>
        </div>
      </div>

      <PageState
        :loading="loading"
        :error="loadError"
        :empty="!channels.length"
        empty-text="暂无渠道"
        @retry="load"
      >
        <div class="channel-list p-2 sm:p-3">
          <article
            v-for="(channel, index) in sortedChannels"
            :key="channel.id"
            class="channel-row group relative grid min-w-0 gap-3 rounded-xl border border-transparent px-3 py-3 sm:grid-cols-[auto_minmax(0,1fr)_auto] sm:items-center sm:px-4"
            :class="[
              channel.status !== 1 ? 'channel-row-off' : '',
              dragIndex === index ? 'channel-row-dragging' : '',
              dropIndex === index && dragIndex !== null && dragIndex !== index ? 'channel-row-dropzone' : '',
            ]"
            :draggable="!reordering && canReorder"
            :style="{ '--row-index': index }"
            @dragstart="onDragStart(index, $event)"
            @dragover.prevent="onDragOver(index)"
            @drop.prevent="onDrop(index)"
            @dragend="onDragEnd"
          >
            <span v-if="dropIndex === index && dragIndex !== null && dragIndex !== index" class="channel-drop-line" aria-hidden="true"></span>

            <div class="flex items-center gap-2 sm:self-stretch">
              <input type="checkbox" :checked="selectedIds.has(channel.id)" :aria-label="`选择渠道 ${channel.name}`" @change="toggleSelected(channel.id)" />
              <button type="button" class="channel-grip" :disabled="reordering || !canReorder" :aria-label="`拖动调整 ${channel.name} 的优先级`"><span aria-hidden="true">⠿</span></button>
              <div class="channel-priority" :title="`优先级 ${index + 1}`">
                <span class="channel-priority-line" aria-hidden="true"></span>
                <b>{{ String(index + 1).padStart(2, '0') }}</b>
              </div>
            </div>

            <div class="min-w-0">
              <div class="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1">
                <span class="channel-state-dot" :class="`channel-state-${breakerState(channel)}`" :title="breakerText(channel)" aria-hidden="true"><i></i></span>
                <button type="button" class="min-w-0 truncate text-left font-cond text-[16px] font-semibold tracking-[0.01em] text-ink transition-colors hover:text-blue" :title="channel.name" @click="openEdit(channel)">{{ channel.name }}</button>
                <span class="font-mono text-[10px] uppercase tracking-[0.1em] text-faint">{{ breakerText(channel) }}</span>
              </div>
              <div class="mt-1 flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1 text-[11px] text-soft">
                <span class="font-mono" :title="channel.base_url">{{ displayEndpoint(channel.base_url) }}</span>
                <span aria-hidden="true" class="text-line">/</span>
                <span>{{ channel.group || 'default' }} 分组</span>
                <span aria-hidden="true" class="text-line">/</span>
                <span>{{ modelCount(channel) }} 个模型</span>
              </div>
              <div class="mt-2 flex flex-wrap items-center gap-1.5">
                <span class="chip chip-blue">{{ typeName(channel.type) }}</span>
                <span class="chip">权重 ×{{ channel.weight }}</span>
                <span class="chip" :class="healthClass(channelHealth(channel))" :title="healthTitle(channelHealth(channel))">{{ healthText(channelHealth(channel)) }}</span>
              </div>
            </div>

            <div class="flex items-center justify-between gap-2 border-t border-line/70 pt-3 sm:justify-end sm:border-0 sm:pt-0">
              <div class="flex items-center gap-2 sm:mr-1">
                <span class="text-[11px] text-soft">{{ channel.status === 1 ? '启用' : '停用' }}</span>
                <button type="button" class="channel-switch" :class="{ 'channel-switch-on': channel.status === 1 }" :disabled="togglingIds.has(channel.id)" :aria-pressed="channel.status === 1" :aria-label="`${channel.status === 1 ? '停用' : '启用'}渠道 ${channel.name}`" @click="toggleChannel(channel)"><span aria-hidden="true"></span></button>
              </div>
              <div class="flex items-center gap-1.5">
                <button type="button" class="btn btn-primary btn-sm whitespace-nowrap" :aria-label="`管理渠道 ${channel.name}`" @click="openEdit(channel)">管理</button>
                <ActionMenu>
                  <button v-if="breakerState(channel) === 'trip'" role="menuitem" type="button" class="text-trip" :disabled="resettingIds.has(channel.id)" @click.stop="resetBreaker(channel)">{{ resettingIds.has(channel.id) ? '解除中' : '解除熔断' }}</button>
                  <button role="menuitem" type="button" :disabled="checkupLoadingId !== null" @click.stop="checkupChannel(channel)">{{ checkupLoadingId === channel.id ? '检查中' : '运行检查' }}</button>
                  <button role="menuitem" type="button" :disabled="togglingIds.has(channel.id)" @click.stop="toggleChannel(channel)">{{ togglingIds.has(channel.id) ? '切换中' : channel.status === 1 ? '停用渠道' : '启用渠道' }}</button>
                  <button role="menuitem" type="button" class="text-trip" :disabled="deletingIds.has(channel.id)" @click.stop="removeChannel(channel)">{{ deletingIds.has(channel.id) ? '删除中' : '删除渠道' }}</button>
                </ActionMenu>
              </div>
            </div>
          </article>
        </div>

        <div v-if="channels.length && !sortedChannels.length" class="m-3 rounded-xl border border-dashed border-line bg-white px-4 py-10 text-center">
          <div class="font-medium text-ink">没有匹配的渠道</div>
          <p class="mt-1 text-xs text-soft">尝试清空搜索词或切换运行状态。</p>
          <button type="button" class="btn btn-sm mt-3" @click="channelQuery = ''; statusFilter = 'all'">清除筛选</button>
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
      <div class="channel-editor min-w-0">
        <div class="editor-mobile-nav" role="tablist" aria-label="渠道配置">
          <button type="button" role="tab" :aria-selected="editorTab === 'connection'" @click="editorTab = 'connection'"><span class="editor-step" :class="editorSteps.connection ? 'editor-step-done' : ''">1</span><span>连接</span></button>
          <button type="button" role="tab" :aria-selected="editorTab === 'models'" @click="editorTab = 'models'"><span class="editor-step" :class="editorSteps.models ? 'editor-step-done' : ''">2</span><span>模型</span></button>
          <button type="button" role="tab" :aria-selected="editorTab === 'routing'" @click="editorTab = 'routing'"><span class="editor-step" :class="editorSteps.routing ? 'editor-step-done' : 'editor-step-warn'">3</span><span>路由</span></button>
        </div>

        <div class="editor-layout">
          <aside class="editor-sidebar" aria-label="渠道配置导航">
            <div class="editor-device-mark">
              <span class="channel-state-dot" :class="form.status === 1 ? 'channel-state-run' : 'channel-state-off'" aria-hidden="true"><i></i></span>
              <div class="min-w-0"><b>{{ form.name || '未命名渠道' }}</b><span>{{ form.id ? `CHANNEL ${form.id}` : 'NEW CHANNEL' }}</span></div>
            </div>
            <nav class="editor-side-nav" role="tablist" aria-label="渠道配置阶段">
              <button type="button" role="tab" :aria-selected="editorTab === 'connection'" @click="editorTab = 'connection'"><span class="editor-nav-index">01</span><span><b>连接与凭据</b><small>{{ editorSteps.connection ? '已配置' : '需要完善' }}</small></span><i :class="editorSteps.connection ? 'is-done' : ''"></i></button>
              <button type="button" role="tab" :aria-selected="editorTab === 'models'" @click="editorTab = 'models'"><span class="editor-nav-index">02</span><span><b>模型工作台</b><small>{{ enabledCount }} 个已启用</small></span><i :class="editorSteps.models ? 'is-done' : ''"></i></button>
              <button type="button" role="tab" :aria-selected="editorTab === 'routing'" @click="editorTab = 'routing'"><span class="editor-nav-index">03</span><span><b>路由与复写</b><small>{{ editorSteps.routing ? '校验通过' : '存在错误' }}</small></span><i :class="editorSteps.routing ? 'is-done' : 'is-error'"></i></button>
            </nav>
            <div class="editor-summary">
              <span>配置摘要</span>
              <dl><div><dt>协议</dt><dd>{{ typeName(form.type) }}</dd></div><div><dt>模型</dt><dd>{{ enabledCount }} / {{ models.length }}</dd></div><div><dt>权重</dt><dd>×{{ form.weight || 1 }}</dd></div><div><dt>请求头</dt><dd>{{ customHeaderCount }}</dd></div></dl>
            </div>
          </aside>

          <main class="editor-workspace">
            <header class="editor-workspace-head">
              <div><span>{{ activeEditor.index }}</span><h3>{{ activeEditor.title }}</h3><p>{{ activeEditor.note }}</p></div>
              <span class="chip" :class="editorSteps[editorTab] ? 'chip-run' : 'chip-test'">{{ editorSteps[editorTab] ? '配置就绪' : '等待完善' }}</span>
            </header>
            <div class="editor-workspace-body">
        <section v-show="editorTab === 'connection'" class="editor-panel" aria-labelledby="nameplate-heading">
          <div class="editor-section-head">
            <div><div id="nameplate-heading" class="dim-title">渠道身份</div><div class="mt-0.5 text-[12px] text-soft">用于路由识别、分组和默认协议判断。</div></div>
            <label class="inline-flex items-center gap-2 text-xs text-soft"><span>{{ form.status === 1 ? '已启用' : '已停用' }}</span><button type="button" class="channel-switch" :class="{ 'channel-switch-on': form.status === 1 }" :aria-pressed="form.status === 1" @click="form.status = form.status === 1 ? 0 : 1"><span></span></button></label>
          </div>
          <div class="grid gap-3 p-4 md:grid-cols-2 xl:grid-cols-4">
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
                  :type="revealKey ? 'text' : 'password'"
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
                <button type="button" class="btn shrink-0" :aria-pressed="revealKey" :aria-label="revealKey ? '隐藏 API Key' : '显示 API Key'" @click="revealKey = !revealKey">{{ revealKey ? '隐藏' : '显示' }}</button>
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

        <section v-show="editorTab === 'models'" class="editor-panel" aria-labelledby="circuits-heading">
          <div class="editor-section-head">
            <div>
              <div id="circuits-heading" class="dim-title">模型清单</div>
              <div class="mt-0.5 text-[12px] text-soft">{{ enabledCount }} 个启用，共 {{ models.length }} 个配置。</div>
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
            <div v-if="batchTesting || batchSummary" class="mb-3 rounded-xl border border-line bg-panel/40 p-3" aria-live="polite">
              <div class="flex flex-wrap items-center justify-between gap-2">
                <div class="flex flex-wrap items-center gap-2">
                  <span v-if="batchTesting" class="chip chip-test">执行中 {{ batchDone }}/{{ batchTotal }}</span>
                  <template v-if="batchSummary">
                    <span class="chip chip-run">通过 {{ batchSummary.success }}</span>
                    <span class="chip chip-trip">失败 {{ batchSummary.failed }}</span>
                    <span class="chip chip-test">总计 {{ batchSummary.total }}</span>
                  </template>
                </div>
                <span v-if="batchTesting" class="font-mono text-[10px] text-soft">{{ batchTotal ? Math.round((batchDone / batchTotal) * 100) : 0 }}%</span>
              </div>
              <div v-if="batchTesting" class="mt-2 h-1.5 overflow-hidden rounded-full bg-white">
                <span class="block h-full rounded-full bg-test transition-[width] duration-300" :style="{ width: `${batchTotal ? (batchDone / batchTotal) * 100 : 0}%` }"></span>
              </div>
            </div>

            <div class="mb-3 flex flex-col gap-2 sm:flex-row">
              <input
                v-model="newModelName"
                class="input input-mono"
                placeholder="模型显示名（可使用 * 通配）"
                aria-label="新模型名称"
                @keyup.enter="addModel"
              />
              <button type="button" class="btn shrink-0" aria-label="添加模型" @click="addModel">添加模型</button>
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

        <div v-show="editorTab === 'routing'" class="editor-routing-grid">
          <section class="editor-panel" aria-labelledby="rules-heading">
            <div class="border-b border-line px-3 py-2.5">
              <div id="rules-heading" class="dim-title">协议路由规则</div>
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
              <button type="button" class="btn btn-sm" aria-label="添加协议规则" @click="rules.push({ pattern: '', protocol: 'anthropic' })">添加规则</button>
            </div>
          </section>

          <section class="editor-panel" aria-labelledby="advanced-heading">
            <div class="border-b border-line px-3 py-2.5">
              <div id="advanced-heading" class="dim-title">权重与请求复写</div>
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
            </div>
          </main>
        </div>

        <div v-if="editorError" class="mt-3 rounded-lg border border-trip bg-trip-wash px-3 py-2 text-[13px] text-trip" role="alert">{{ editorError }}</div>
      </div>

      <template #footer>
        <div class="flex w-full flex-wrap items-center justify-between gap-2">
          <div>
            <span class="block font-mono text-2xs text-faint">{{ form.id ? `CHANNEL ID ${form.id}` : 'UNSAVED CHANNEL' }}</span>
            <span class="mt-0.5 block text-[11px]" :class="canSave ? 'text-run' : 'text-soft'">{{ saveHint }}</span>
          </div>
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

<style scoped>
.channel-list {
  background:
    linear-gradient(90deg, transparent 27px, rgba(53, 100, 212, .08) 27px, rgba(53, 100, 212, .08) 28px, transparent 28px),
    #f8fafd;
}

.route-bus {
  display: flex;
  min-height: 70px;
  gap: 4px;
}

.route-bus-segment {
  --segment-color: #94a0b2;
  flex: var(--segment-grow) 1 0;
  min-width: 88px;
  border: 1px solid #dde4ed;
  border-radius: 10px;
  background: #fff;
  padding: 10px 12px;
  text-align: left;
  transition: flex-grow 240ms cubic-bezier(.2,.8,.2,1), border-color 180ms ease, background-color 180ms ease, transform 180ms ease;
}
.route-bus-segment:hover { transform: translateY(-2px); border-color: var(--segment-color); }
.route-bus-active { background: color-mix(in srgb, var(--segment-color) 8%, white); border-color: var(--segment-color); box-shadow: 0 8px 22px color-mix(in srgb, var(--segment-color) 14%, transparent); }
.route-bus-run { --segment-color: #23877f; }
.route-bus-test { --segment-color: #b7791f; }
.route-bus-trip { --segment-color: #d05a52; }
.route-bus-off { --segment-color: #94a0b2; }
.route-bus-line { display: block; height: 3px; border-radius: 999px; background: var(--segment-color); transform-origin: left; transition: transform 220ms ease; }
.route-bus-segment:hover .route-bus-line, .route-bus-active .route-bus-line { transform: scaleX(.82); }
.route-bus-copy { display: flex; align-items: baseline; justify-content: space-between; gap: 8px; margin-top: 9px; }
.route-bus-copy b { font-family: 'Saira SemiCondensed', sans-serif; font-size: 22px; line-height: 1; color: #18243a; }
.route-bus-copy span { font-family: 'Spline Sans Mono', monospace; font-size: 9px; color: #627087; white-space: nowrap; }

.channel-row {
  background: rgba(255,255,255,.96);
  transition: border-color 180ms ease, box-shadow 180ms ease, transform 180ms ease, opacity 180ms ease;
  animation: feeder-in 360ms both cubic-bezier(.2,.8,.2,1);
  animation-delay: min(calc(var(--row-index) * 32ms), 240ms);
}
.channel-row:hover { border-color: #d2dce9; transform: translateX(2px); box-shadow: 0 8px 24px rgba(22,36,58,.06); }
.channel-row-off { opacity: .68; filter: saturate(.7); }
.channel-row.channel-row-dragging { opacity: .48; transform: scale(.99); box-shadow: 0 18px 40px rgba(15,23,42,.18); cursor: grabbing; }
.channel-row.channel-row-dropzone { background: #edf2ff; border-color: #88a5ea; }
.channel-drop-line { position: absolute; inset-inline: 8px; top: -5px; height: 3px; border-radius: 999px; background: #3564d4; box-shadow: 0 0 0 4px rgba(53,100,212,.10); }
.channel-grip { display: inline-flex; width: 24px; height: 30px; align-items: center; justify-content: center; border-radius: 6px; color: #94a0b2; cursor: grab; transition: color 150ms ease, background-color 150ms ease; }
.channel-grip:hover:not(:disabled) { color: #3564d4; background: #edf2ff; }
.channel-grip:active:not(:disabled) { cursor: grabbing; }
.channel-grip:disabled { cursor: not-allowed; opacity: .35; }
.channel-priority { display: flex; align-items: center; gap: 7px; min-width: 42px; font-family: 'Spline Sans Mono', monospace; font-size: 10px; color: #94a0b2; }
.channel-priority-line { width: 12px; height: 1px; background: #cbd5e1; }
.channel-state-dot { position: relative; display: inline-flex; width: 10px; height: 10px; flex: 0 0 auto; border-radius: 999px; background: #94a0b2; }
.channel-state-dot i { position: absolute; inset: 3px; border-radius: inherit; background: white; opacity: .72; }
.channel-state-run { background: #23877f; box-shadow: 0 0 0 4px rgba(35,135,127,.11); }
.channel-state-test { background: #b7791f; box-shadow: 0 0 0 4px rgba(183,121,31,.11); animation: signal-pulse 1.8s ease-in-out infinite; }
.channel-state-trip { background: #d05a52; box-shadow: 0 0 0 4px rgba(208,90,82,.12); animation: signal-pulse 1.35s ease-in-out infinite; }
.channel-state-off { background: #94a0b2; }

.channel-editor { margin: -20px; background: #f4f7fb; }
.editor-layout { display: grid; grid-template-columns: 224px minmax(0, 1fr); min-height: 610px; }
.editor-sidebar { display: flex; flex-direction: column; border-right: 1px solid #dde4ed; background: #16243a; color: white; }
.editor-device-mark { display: flex; align-items: center; gap: 11px; padding: 20px 18px; border-bottom: 1px solid rgba(255,255,255,.09); }
.editor-device-mark b { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-family: 'Saira SemiCondensed', sans-serif; font-size: 15px; font-weight: 600; color: rgba(255,255,255,.92); }
.editor-device-mark span:not(.channel-state-dot) { display: block; margin-top: 2px; font-family: 'Spline Sans Mono', monospace; font-size: 9px; letter-spacing: .12em; color: rgba(255,255,255,.34); }
.editor-side-nav { display: grid; gap: 4px; padding: 14px 10px; }
.editor-side-nav button { position: relative; display: grid; grid-template-columns: 27px minmax(0,1fr) 8px; align-items: center; gap: 8px; min-height: 58px; border-radius: 10px; padding: 9px 10px; text-align: left; color: rgba(255,255,255,.48); transition: background-color 160ms ease, color 160ms ease, transform 160ms ease; }
.editor-side-nav button:hover { color: rgba(255,255,255,.82); background: rgba(255,255,255,.05); transform: translateX(2px); }
.editor-side-nav button[aria-selected='true'] { color: white; background: rgba(255,255,255,.09); box-shadow: inset 0 0 0 1px rgba(255,255,255,.06); }
.editor-nav-index { font-family: 'Spline Sans Mono', monospace; font-size: 9px; color: rgba(142,177,255,.75); }
.editor-side-nav b { display: block; font-size: 12px; font-weight: 600; }
.editor-side-nav small { display: block; margin-top: 3px; font-size: 10px; color: rgba(255,255,255,.32); }
.editor-side-nav i { width: 7px; height: 7px; border: 1px solid rgba(255,255,255,.25); border-radius: 999px; }
.editor-side-nav i.is-done { border-color: #60c3b8; background: #60c3b8; box-shadow: 0 0 0 3px rgba(96,195,184,.1); }
.editor-side-nav i.is-error { border-color: #ed8b83; background: #ed8b83; }
.editor-summary { margin-top: auto; border-top: 1px solid rgba(255,255,255,.09); padding: 16px 18px 18px; }
.editor-summary > span { font-family: 'Spline Sans Mono', monospace; font-size: 9px; text-transform: uppercase; letter-spacing: .14em; color: rgba(255,255,255,.3); }
.editor-summary dl { display: grid; gap: 8px; margin-top: 10px; }
.editor-summary dl div { display: flex; justify-content: space-between; gap: 10px; font-size: 10px; }
.editor-summary dt { color: rgba(255,255,255,.34); }
.editor-summary dd { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: rgba(255,255,255,.72); font-family: 'Spline Sans Mono', monospace; }
.editor-workspace { min-width: 0; background: #f4f7fb; }
.editor-workspace-head { display: flex; align-items: center; justify-content: space-between; gap: 16px; border-bottom: 1px solid #dde4ed; background: rgba(255,255,255,.82); padding: 18px 22px; }
.editor-workspace-head > div > span { float: left; margin-right: 10px; padding-top: 3px; font-family: 'Spline Sans Mono', monospace; font-size: 9px; color: #3564d4; }
.editor-workspace-head h3 { font-family: 'Saira SemiCondensed', sans-serif; font-size: 20px; font-weight: 600; line-height: 1.1; color: #18243a; }
.editor-workspace-head p { margin-top: 4px; font-size: 11px; color: #627087; }
.editor-workspace-body { padding: 18px; }
.editor-panel { overflow: hidden; border: 1px solid #dde4ed; border-radius: 12px; background: white; box-shadow: 0 1px 2px rgba(22,36,58,.03); }
.editor-section-head { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 8px; border-bottom: 1px solid #dde4ed; padding: 13px 16px; }
.editor-routing-grid { display: grid; gap: 14px; }
.editor-mobile-nav { display: none; }
.editor-step { display: inline-flex; width: 20px; height: 20px; align-items: center; justify-content: center; border: 1px solid #cbd5e1; border-radius: 999px; font-family: 'Spline Sans Mono', monospace; font-size: 9px; color: #94a0b2; background: white; }
.editor-step-done { color: white; border-color: #23877f; background: #23877f; }
.editor-step-warn { border-color: #d05a52; color: #d05a52; }

@keyframes feeder-in { from { opacity: 0; transform: translateY(8px); } to { opacity: 1; transform: translateY(0); } }
@keyframes signal-pulse { 50% { box-shadow: 0 0 0 7px rgba(183,121,31,0); } }

@media (max-width: 900px) {
  .channel-editor { margin: -16px; }
  .editor-layout { display: block; min-height: 0; }
  .editor-sidebar { display: none; }
  .editor-mobile-nav { display: grid; grid-template-columns: repeat(3, minmax(0,1fr)); gap: 4px; border-bottom: 1px solid #dde4ed; background: white; padding: 8px; }
  .editor-mobile-nav button { display: flex; min-width: 0; align-items: center; justify-content: center; gap: 6px; border-radius: 8px; padding: 7px 5px; font-size: 11px; font-weight: 600; color: #627087; }
  .editor-mobile-nav button[aria-selected='true'] { background: #16243a; color: white; }
  .editor-workspace-head { padding: 14px 16px; }
  .editor-workspace-body { padding: 12px; }
}

@media (max-width: 700px) {
  .route-bus { overflow-x: auto; padding-bottom: 4px; scroll-snap-type: x proximity; }
  .route-bus-segment { flex: 0 0 126px; scroll-snap-align: start; }
  .channel-list { background-position-x: -5px; }
  .channel-row:hover { transform: none; }
  .editor-workspace-head .chip { display: none; }
}

@media (prefers-reduced-motion: reduce) {
  .channel-row, .channel-state-test, .channel-state-trip { animation: none; }
  .route-bus-segment, .route-bus-line { transition: none; }
}
</style>
