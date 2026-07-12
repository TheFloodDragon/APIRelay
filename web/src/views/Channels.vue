<script setup>
import { computed, getCurrentInstance, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import api, { copyText } from '../api'
import { confirmAction } from '../composables/useConfirm'
import { DEFAULT_HEALTH_CONFIG, hasHealth, healthTotal, healthText, healthTitle, healthClass as healthClassBy } from '../health'
import Modal from '../components/Modal.vue'
import Drawer from '../components/Drawer.vue'
import PageState from '../components/PageState.vue'
import PageHeader from '../components/PageHeader.vue'
import ConsoleSection from '../components/ConsoleSection.vue'
import DataToolbar from '../components/DataToolbar.vue'
import InlineNotice from '../components/InlineNotice.vue'
import ConsoleIcon from '../components/ConsoleIcon.vue'
import HeaderOverrideEditor from '../components/HeaderOverrideEditor.vue'
import BodyOverrideEditor from '../components/BodyOverrideEditor.vue'
import ChannelConsoleHeader from '../components/ChannelConsoleHeader.vue'

const { proxy } = getCurrentInstance()
const route = useRoute()
const router = useRouter()
const notify = (message, type = 'info', duration) => proxy?.$toast?.add(message, type, duration)

const channels = ref([])
const channelTypes = ref([])
const protocols = ref([])
const loading = ref(true)
const loadError = ref('')
const metadataLoading = ref(false)
const isMobile = ref(typeof window !== 'undefined' ? window.matchMedia('(max-width: 767px)').matches : false)
const selectedChannelId = ref(null)
const previousSelectedId = ref(null)
const editorBaseline = ref('')
let mobileMediaQuery = null

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
  overrides: headerValidation.value.valid && bodyValidation.value.valid,
  reliability: true,
}))
const editorSections = computed(() => [
  { key: 'connection', index: '01', label: '连接与身份', note: editorSteps.value.connection ? '连接信息完整' : '需要完善凭据', icon: 'key' },
  { key: 'models', index: '02', label: '模型与价格', note: `${enabledCount.value} 个模型已启用`, icon: 'models' },
  { key: 'overrides', index: '03', label: '请求改写', note: editorSteps.value.overrides ? 'JSON 校验通过' : '存在配置错误', icon: 'command' },
  { key: 'reliability', index: '04', label: '可靠性', note: form.value.status === 1 ? '渠道参与路由' : '渠道已停用', icon: 'shield' },
])
const activeEditor = computed(() => editorSections.value.find((item) => item.key === editorTab.value) || editorSections.value[0])
const customHeaderCount = computed(() => headerValidation.value.valid ? headerValidation.value.allowedCount : 0)
const checkupRate = computed(() => {
  const summary = checkupSummary.value
  if (!summary?.total) return 0
  return Math.round((summary.success / summary.total) * 100)
})
const testRecordRows = computed(() => models.value
  .map((model) => ({ model, result: testResults.value[model.name] }))
  .filter((row) => row.result || testing.value[row.model.name]))
const isDirty = computed(() => showEditor.value && editorBaseline.value !== editorSnapshot())
const saveStatus = computed(() => {
  if (saving.value) return { label: '保存中', tone: 'saving' }
  if (isDirty.value) return { label: '有未保存更改', tone: 'dirty' }
  if (form.value.id) return { label: '已保存 · 自动保存关闭', tone: 'saved' }
  return { label: '新渠道 · 自动保存关闭', tone: 'idle' }
})
const selectedChannel = computed(() => channels.value.find((channel) => channel.id === selectedChannelId.value) || null)

function editorSnapshot() {
  return JSON.stringify({ form: form.value, models: models.value, rules: rules.value })
}

function markEditorBaseline() {
  editorBaseline.value = editorSnapshot()
}

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
    editorTab.value = 'overrides'
    editorError.value = `无法${action}：${headerValidation.value.error}`
    notify(editorError.value, 'warn')
    return false
  }
  if (!bodyValidation.value.valid) {
    editorTab.value = 'overrides'
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
    const preferred = channels.value.find((channel) => channel.id === selectedChannelId.value)
    if (!preferred && selectedChannelId.value !== null) {
      selectedChannelId.value = null
      showEditor.value = false
    }
    if (!isMobile.value && !showEditor.value && channels.value.length) {
      openEdit(preferred || channels.value[0], { remember: false })
    }
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

async function confirmDiscardChanges() {
  if (!isDirty.value) return true
  return confirmAction({
    title: '放弃未保存更改',
    message: '当前渠道配置尚未保存。继续后这些更改会丢失。',
    confirmLabel: '放弃更改',
  })
}

async function openCreate() {
  if (!(await confirmDiscardChanges())) return
  previousSelectedId.value = selectedChannelId.value
  selectedChannelId.value = null
  form.value = blank()
  models.value = []
  rules.value = []
  resetEditorState()
  const type = channelTypes.value.find((item) => item.value === form.value.type)
  if (type) form.value.base_url = type.default_base_url
  showEditor.value = true
  markEditorBaseline()
}

async function openEdit(channel, options = {}) {
  if (!channel) return
  const changingChannel = selectedChannelId.value !== channel.id || !form.value.id
  if (!changingChannel && showEditor.value && !options.force) return
  if (changingChannel && !(await confirmDiscardChanges())) return
  if (options.remember !== false && selectedChannelId.value !== channel.id) previousSelectedId.value = selectedChannelId.value
  selectedChannelId.value = channel.id
  form.value = { ...blank(), ...channel, key: channel.key || '' }
  models.value = (Array.isArray(channel._models) ? channel._models : parseModels(channel)).map((item) => ({ ...item }))
  rules.value = parseRules(channel)
  resetEditorState()
  showEditor.value = true
  markEditorBaseline()
}

async function closeEditor() {
  if (editorBusy.value || !(await confirmDiscardChanges())) return
  editorBaseline.value = editorSnapshot()
  if (isMobile.value) {
    showEditor.value = false
    return
  }
  const current = channels.value.find((channel) => channel.id === selectedChannelId.value)
  if (current) {
    await openEdit(current, { remember: false, force: true })
    return
  }
  const fallback = channels.value.find((channel) => channel.id === previousSelectedId.value) || channels.value[0]
  if (fallback) await openEdit(fallback, { remember: false })
  else showEditor.value = false
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
    const originalId = form.value.id
    let response
    if (originalId) {
      response = await api.put(`/channels/${originalId}`, payload)
      notify('渠道已更新', 'success')
    } else {
      response = await api.post('/channels', payload)
      notify('渠道已创建', 'success')
    }
    editorBaseline.value = editorSnapshot()
    selectedChannelId.value = response?.id || originalId || null
    showEditor.value = false
    await load()
    const savedChannel = channels.value.find((channel) => channel.id === selectedChannelId.value)
      || channels.value.find((channel) => channel.name === payload.name)
    if (savedChannel) selectedChannelId.value = savedChannel.id
    if (!isMobile.value && savedChannel && form.value.id !== savedChannel.id) await openEdit(savedChannel, { remember: false })
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
  const confirmed = await confirmAction({
    title: '批量删除渠道',
    message: `确认删除选中的 ${ids.length} 个渠道？\n\n渠道及其模型路由将一并删除，此操作不可撤销。`,
    confirmLabel: `删除 ${ids.length} 个渠道`,
  })
  if (!confirmed) return
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
  const syncEditor = form.value.id === channel.id
  const editorWasDirty = isDirty.value
  updateSet(togglingIds, channel.id, true)
  channel.status = nextStatus
  if (syncEditor) form.value.status = nextStatus
  try {
    await api.patch(`/channels/${channel.id}/status`, { enabled: nextStatus === 1 })
    if (syncEditor && !editorWasDirty) markEditorBaseline()
    notify(`「${channel.name}」已${nextStatus === 1 ? '启用' : '停用'}`, 'success')
  } catch (error) {
    channel.status = previous
    if (syncEditor) form.value.status = previous
    notify(`状态切换失败：${error.message}`, 'error')
  } finally {
    updateSet(togglingIds, channel.id, false)
  }
}

async function removeChannel(channel) {
  if (deletingIds.value.has(channel.id)) return
  const confirmed = await confirmAction({
    title: '删除渠道',
    message: `确认删除渠道「${channel.name}」？\n\n此操作不可撤销。`,
    confirmLabel: '删除渠道',
  })
  if (!confirmed) return
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
  const editorWasDirty = isDirty.value
  updateSet(resettingIds, channel.id, true)
  try {
    await api.post(`/channels/${channel.id}/health/reset`)
    channel.cooldown_until = 0
    if (form.value.id === channel.id) {
      form.value.cooldown_until = 0
      if (!editorWasDirty) markEditorBaseline()
    }
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

function handleViewportChange(event) {
  isMobile.value = event.matches
  if (!isMobile.value && !showEditor.value && channels.value.length) {
    openEdit(selectedChannel.value || channels.value[0], { remember: false })
  }
}

onMounted(async () => {
  mobileMediaQuery = window.matchMedia('(max-width: 767px)')
  isMobile.value = mobileMediaQuery.matches
  mobileMediaQuery.addEventListener?.('change', handleViewportChange)
  await Promise.all([loadMeta(), load()])
  if (route.query.action === 'new') {
    await openCreate()
    const query = { ...route.query }
    delete query.action
    router.replace({ query })
  }
})

onBeforeUnmount(() => mobileMediaQuery?.removeEventListener?.('change', handleViewportChange))
</script>

<template>
  <div class="page-workbench channels-page min-w-0">
    <PageHeader eyebrow="上游路由" title="上游渠道" description="在同一工作台中编排渠道优先级，并维护连接、模型、请求改写与可靠性策略。">
      <template #actions>
        <button type="button" class="btn" :disabled="loading" aria-label="刷新渠道列表" @click="load">
          <ConsoleIcon name="arrowPath" class="h-4 w-4" :class="{ 'animate-spin': loading }" />
          {{ loading ? '刷新中' : '刷新' }}
        </button>
        <button type="button" class="btn btn-primary" :disabled="metadataLoading" aria-label="新建渠道" @click="openCreate">
          <ConsoleIcon name="plus" class="h-4 w-4" />新建渠道
        </button>
      </template>
    </PageHeader>

    <div class="channels-workspace">
      <aside class="channel-master sheet min-w-0" aria-label="渠道队列">
        <ChannelConsoleHeader
          v-model:query="channelQuery" v-model:status="statusFilter"
          :summary="channelSummary" :segments="routeSegments"
          :selected-count="selectedIds.size" :bulk-deleting="bulkDeleting"
          :reordering="reordering" :visible-count="sortedChannels.length"
          @bulk-delete="bulkDeleteChannels"
        />
        <div class="channel-list-scroll">
          <PageState :loading="loading" :error="loadError" :empty="!channels.length" empty-text="暂无渠道" @retry="load">
            <div class="channel-list p-2">
              <article
                v-for="(channel, index) in sortedChannels" :key="channel.id"
                class="channel-row relative min-w-0 rounded-lg border p-3"
                :class="[
                  selectedChannelId === channel.id ? 'channel-row-selected' : '',
                  channel.status !== 1 ? 'channel-row-off' : '',
                  dragIndex === index ? 'channel-row-dragging' : '',
                  dropIndex === index && dragIndex !== null && dragIndex !== index ? 'channel-row-dropzone' : '',
                ]"
                :draggable="!reordering && canReorder" :style="{ '--row-index': index }"
                @click="openEdit(channel)" @dragstart="onDragStart(index, $event)"
                @dragover.prevent="onDragOver(index)" @drop.prevent="onDrop(index)" @dragend="onDragEnd"
              >
                <span v-if="dropIndex === index && dragIndex !== null && dragIndex !== index" class="channel-drop-line" aria-hidden="true"></span>
                <div class="flex min-w-0 items-start gap-2.5">
                  <input type="checkbox" class="mt-1 shrink-0" :checked="selectedIds.has(channel.id)" :aria-label="`选择渠道 ${channel.name}`" @click.stop @change="toggleSelected(channel.id)" />
                  <button type="button" class="channel-grip mt-0.5 shrink-0" :disabled="reordering || !canReorder" :aria-label="`拖动调整 ${channel.name} 的优先级`" @click.stop><ConsoleIcon name="bars" class="h-4 w-4" /></button>
                  <div class="min-w-0 flex-1">
                    <div class="flex min-w-0 items-center gap-2">
                      <span class="channel-state-dot" :class="`channel-state-${breakerState(channel)}`" :title="breakerText(channel)" aria-hidden="true"><i></i></span>
                      <button type="button" class="min-w-0 flex-1 truncate text-left text-sm font-semibold text-ink" :title="channel.name" @click.stop="openEdit(channel)">{{ channel.name }}</button>
                      <span class="font-mono text-[9px] text-faint">{{ String(index + 1).padStart(2, '0') }}</span>
                    </div>
                    <div class="mt-1 truncate font-mono text-[10px] text-soft" :title="channel.base_url">{{ displayEndpoint(channel.base_url) }}</div>
                    <div class="mt-2 flex min-w-0 flex-wrap items-center gap-1.5">
                      <span class="chip" :class="breakerState(channel) === 'run' ? 'chip-run' : breakerState(channel) === 'trip' ? 'chip-trip' : 'chip-test'">{{ breakerText(channel) }}</span>
                      <span class="chip">{{ modelCount(channel) }} 模型</span>
                      <span class="chip" :class="healthClass(channelHealth(channel))" :title="healthTitle(channelHealth(channel))">{{ healthText(channelHealth(channel)) }}</span>
                    </div>
                  </div>
                </div>
                <div class="mt-3 flex min-w-0 items-center justify-between gap-2 border-t border-line/70 pt-2.5">
                  <div class="min-w-0 truncate text-[10px] text-soft">{{ channel.group || 'default' }} · {{ typeName(channel.type) }} · 权重 ×{{ channel.weight }}</div>
                  <div class="flex shrink-0 items-center gap-1">
                    <button type="button" class="icon-btn h-8 w-8" :disabled="checkupLoadingId !== null" :aria-label="`检查渠道 ${channel.name}`" :title="checkupLoadingId === channel.id ? '检查中' : '运行检查'" @click.stop="checkupChannel(channel)"><ConsoleIcon name="bolt" class="h-4 w-4" :class="{ 'animate-pulse': checkupLoadingId === channel.id }" /></button>
                    <button type="button" class="channel-switch" :class="{ 'channel-switch-on': channel.status === 1 }" :disabled="togglingIds.has(channel.id)" :aria-pressed="channel.status === 1" :aria-label="`${channel.status === 1 ? '停用' : '启用'}渠道 ${channel.name}`" @click.stop="toggleChannel(channel)"><span aria-hidden="true"></span></button>
                    <ConsoleIcon name="chevronRight" class="h-4 w-4 text-faint" />
                  </div>
                </div>
              </article>
            </div>
            <div v-if="channels.length && !sortedChannels.length" class="m-3 rounded-lg border border-dashed border-line bg-surface px-4 py-10 text-center">
              <div class="font-medium text-ink">没有匹配的渠道</div><p class="mt-1 text-xs text-soft">尝试清空搜索词或切换运行状态。</p>
              <button type="button" class="btn btn-sm mt-3" @click="channelQuery = ''; statusFilter = 'all'">清除筛选</button>
            </div>
          </PageState>
        </div>
      </aside>

      <section v-if="!isMobile && !showEditor" class="channel-detail-empty sheet" aria-label="渠道详情空状态">
        <div class="max-w-sm text-center">
          <span class="mx-auto flex h-12 w-12 items-center justify-center rounded-full border border-line bg-surface text-blue-grid"><ConsoleIcon name="server" class="h-6 w-6" /></span>
          <h2 class="mt-4 text-base font-semibold text-ink">选择一个渠道开始配置</h2>
          <p class="mt-2 text-sm leading-6 text-soft">从左侧队列选择已有渠道，或新建渠道并在此完成配置。</p>
          <button type="button" class="btn btn-primary mt-4" @click="openCreate"><ConsoleIcon name="plus" class="h-4 w-4" />新建渠道</button>
        </div>
      </section>

      <component
        :is="isMobile ? Drawer : 'section'" v-if="isMobile || showEditor"
        v-bind="isMobile ? { open: showEditor, title: form.id ? `渠道详情 · ${form.name || '未命名'}` : '新建渠道', width: 'max-w-none', persistent: editorBusy } : { class: 'channel-detail-host sheet' }"
        @close="closeEditor"
      >
        <div v-if="showEditor" class="channel-detail min-w-0">
          <header class="channel-detail-heading">
            <div class="min-w-0">
              <div class="flex min-w-0 flex-wrap items-center gap-2">
                <span class="channel-state-dot" :class="form.status === 1 ? 'channel-state-run' : 'channel-state-off'" aria-hidden="true"><i></i></span>
                <h2 class="min-w-0 truncate text-base font-semibold text-ink">{{ form.name || '未命名渠道' }}</h2><span class="chip">{{ form.id ? `ID ${form.id}` : '新建' }}</span>
              </div>
              <p class="mt-1 truncate font-mono text-[10px] text-soft">{{ displayEndpoint(form.base_url) }}</p>
            </div>
            <div class="hidden shrink-0 items-center gap-2 sm:flex"><span class="chip chip-blue">{{ typeName(form.type) }}</span><span class="chip">{{ enabledCount }} / {{ models.length }} 模型</span></div>
          </header>

          <nav class="detail-mobile-nav" role="tablist" aria-label="渠道配置区域">
            <button v-for="section in editorSections" :key="`mobile-${section.key}`" type="button" role="tab" :aria-selected="editorTab === section.key" @click="editorTab = section.key"><ConsoleIcon :name="section.icon" class="h-4 w-4" /><span>{{ section.label }}</span></button>
          </nav>

          <div class="channel-detail-layout">
            <aside class="channel-detail-nav" aria-label="渠道配置导航">
              <div class="px-3 pb-2 pt-3 font-mono text-[9px] uppercase tracking-[.14em] text-faint">配置区域</div>
              <nav class="space-y-1" role="tablist">
                <button v-for="section in editorSections" :key="section.key" type="button" role="tab" :aria-selected="editorTab === section.key" @click="editorTab = section.key">
                  <ConsoleIcon :name="section.icon" class="h-4 w-4 shrink-0" />
                  <span class="min-w-0 flex-1"><b>{{ section.label }}</b><small>{{ section.note }}</small></span>
                  <span class="detail-nav-state" :class="editorSteps[section.key] ? 'is-done' : 'is-pending'"></span>
                </button>
              </nav>
              <dl class="detail-summary"><div><dt>协议</dt><dd>{{ typeName(form.type) }}</dd></div><div><dt>模型</dt><dd>{{ enabledCount }} / {{ models.length }}</dd></div><div><dt>权重</dt><dd>×{{ form.weight || 1 }}</dd></div><div><dt>请求头</dt><dd>{{ customHeaderCount }}</dd></div></dl>
            </aside>

            <main class="channel-detail-content">
              <div class="mb-3 flex min-w-0 items-start justify-between gap-3">
                <div class="min-w-0"><div class="flex items-center gap-2 font-mono text-[9px] uppercase tracking-[.12em] text-blue-grid"><span>{{ activeEditor.index }}</span><span>{{ activeEditor.label }}</span></div><p class="mt-1 text-xs text-soft">{{ activeEditor.note }}</p></div>
                <span class="chip shrink-0" :class="editorSteps[editorTab] ? 'chip-run' : 'chip-test'">{{ editorSteps[editorTab] ? '区域就绪' : '待完善' }}</span>
              </div>
              <InlineNotice v-if="editorError" class="mb-3" tone="danger" title="无法完成操作">{{ editorError }}</InlineNotice>

              <div v-show="editorTab === 'connection'" class="space-y-3">
                <ConsoleSection title="连接与身份" description="定义渠道名称、默认协议、上游地址与凭据。" eyebrow="Connection">
                  <div class="grid min-w-0 gap-4 md:grid-cols-2">
                    <div><label class="field-label" for="channel-name">渠道名称 *</label><input id="channel-name" v-model="form.name" class="input" placeholder="例：OpenAI 主账号" autocomplete="off" data-autofocus /></div>
                    <div><label class="field-label" for="channel-group">分组</label><input id="channel-group" v-model="form.group" class="input input-mono" placeholder="default" autocomplete="off" /></div>
                    <div><label class="field-label" for="channel-type">默认协议 *</label><select id="channel-type" v-model.number="form.type" class="input" @change="onTypeChange"><option v-for="type in channelTypes" :key="type.value" :value="type.value">{{ type.name }}</option></select></div>
                    <div class="md:col-span-2"><label class="field-label" for="channel-url">Base URL *</label><input id="channel-url" v-model="form.base_url" class="input input-mono" placeholder="https://api.openai.com" autocomplete="off" /></div>
                    <div class="md:col-span-2">
                      <label class="field-label" for="channel-key">API Key *</label>
                      <div class="channel-key-row">
                        <input id="channel-key" v-model="form.key" :type="revealKey ? 'text' : 'password'" class="input input-mono min-w-0" placeholder="upstream-key" name="apirelay-upstream-key" autocomplete="off" autocapitalize="off" autocorrect="off" spellcheck="false" data-1p-ignore data-lpignore="true" data-form-type="other" />
                        <button type="button" class="btn shrink-0" :aria-pressed="revealKey" :aria-label="revealKey ? '隐藏 API Key' : '显示 API Key'" @click="revealKey = !revealKey"><ConsoleIcon :name="revealKey ? 'x' : 'key'" class="h-4 w-4" />{{ revealKey ? '隐藏' : '显示' }}</button>
                        <button type="button" class="btn shrink-0" :disabled="!form.key" @click="copyKey(form)">复制</button>
                      </div>
                    </div>
                  </div>
                </ConsoleSection>
              </div>

              <div v-show="editorTab === 'models'" class="space-y-3">
                <ConsoleSection title="模型与价格" :description="`${enabledCount} 个启用，共 ${models.length} 个配置；价格单位为 USD / 1M tokens。`" eyebrow="Models" flush>
                  <template #actions>
                    <button type="button" class="btn btn-sm" :disabled="editorBusy || !form.base_url || !form.key || enabledCount === 0" @click="testAllInModal"><ConsoleIcon name="bolt" class="h-4 w-4" />{{ batchTesting ? `批测中 ${batchDone}/${batchTotal}` : '批量测试' }}</button>
                    <button type="button" class="btn btn-sm" :disabled="editorBusy || !form.base_url || !form.key" @click="fetchModels"><ConsoleIcon name="arrowPath" class="h-4 w-4" :class="{ 'animate-spin': probing }" />{{ probing ? '探测中' : '探测模型' }}</button>
                  </template>
                  <div class="p-3 sm:p-4">
                    <InlineNotice v-if="batchTesting || batchSummary" class="mb-3" :tone="batchSummary?.failed ? 'warning' : 'info'" title="批量测试"><span v-if="batchTesting">执行中 {{ batchDone }} / {{ batchTotal }}</span><span v-else-if="batchSummary">通过 {{ batchSummary.success }}，失败 {{ batchSummary.failed }}，总计 {{ batchSummary.total }}。</span></InlineNotice>
                    <DataToolbar label="添加模型">
                      <input v-model="newModelName" class="input input-mono min-w-0 flex-1" placeholder="模型显示名（可使用 * 通配）" aria-label="新模型名称" @keyup.enter="addModel" />
                      <template #actions><button type="button" class="btn btn-primary btn-sm" @click="addModel"><ConsoleIcon name="plus" class="h-4 w-4" />添加模型</button></template>
                    </DataToolbar>
                    <div v-if="models.length" class="model-table-wrap mt-3 hidden lg:block">
                      <table class="table-eng min-w-[820px]" aria-label="模型配置表">
                        <thead><tr><th class="w-16">启用</th><th>模型名称</th><th class="w-36">协议</th><th>上游映射</th><th class="w-24 text-right">输入价</th><th class="w-24 text-right">输出价</th><th class="w-20 text-right">测试</th><th class="w-20 text-right">删除</th></tr></thead>
                        <tbody><tr v-for="(model, index) in models" :key="index">
                          <td><button type="button" class="channel-switch" :class="{ 'channel-switch-on': model.enabled }" :aria-pressed="model.enabled" @click="model.enabled = !model.enabled"><span></span></button></td>
                          <td><input v-model="model.name" class="input input-mono py-1 text-[12px]" placeholder="显示名" /></td>
                          <td><select v-model="model.protocol" class="input py-1 text-[12px]"><option value="">继承规则</option><option v-for="protocol in protocols" :key="protocol.value" :value="protocol.value">{{ protocol.name }}</option></select></td>
                          <td><input v-model="model.upstream" class="input input-mono py-1 text-[12px]" placeholder="留空则同显示名" /></td>
                          <td><input v-model.number="model.input" type="number" step="0.01" min="0" class="input py-1 text-right font-mono text-[12px]" placeholder="0" /></td>
                          <td><input v-model.number="model.output" type="number" step="0.01" min="0" class="input py-1 text-right font-mono text-[12px]" placeholder="0" /></td>
                          <td class="text-right"><button type="button" class="btn btn-sm" :disabled="Boolean(testing[model.name]) || batchTesting || saving || !model.name.trim()" @click="testModel(model)">{{ testing[model.name] ? '测试中' : '单测' }}</button></td>
                          <td class="text-right"><button type="button" class="btn btn-danger btn-sm" :disabled="Boolean(testing[model.name]) || batchTesting" @click="removeModel(index, model)">删除</button></td>
                        </tr></tbody>
                      </table>
                    </div>
                    <div v-if="models.length" class="mt-3 grid gap-3 lg:hidden">
                      <article v-for="(model, index) in models" :key="index" class="rounded-lg border border-line bg-surface p-3">
                        <div class="flex items-center justify-between gap-2"><span class="font-medium">模型 {{ index + 1 }}</span><button type="button" class="channel-switch" :class="{ 'channel-switch-on': model.enabled }" :aria-pressed="model.enabled" @click="model.enabled = !model.enabled"><span></span></button></div>
                        <div class="mt-3 grid gap-2">
                          <input v-model="model.name" class="input input-mono" placeholder="模型名称" />
                          <select v-model="model.protocol" class="input"><option value="">继承规则</option><option v-for="protocol in protocols" :key="protocol.value" :value="protocol.value">{{ protocol.name }}</option></select>
                          <input v-model="model.upstream" class="input input-mono" placeholder="上游映射，留空则同模型名称" />
                          <div class="grid grid-cols-2 gap-2"><input v-model.number="model.input" type="number" step="0.01" min="0" class="input" placeholder="输入价" /><input v-model.number="model.output" type="number" step="0.01" min="0" class="input" placeholder="输出价" /></div>
                          <div class="grid grid-cols-2 gap-2"><button type="button" class="btn btn-sm" :disabled="Boolean(testing[model.name]) || batchTesting || saving || !model.name.trim() || !headerValidation.valid" @click="testModel(model)">{{ testing[model.name] ? '测试中' : '单测' }}</button><button type="button" class="btn btn-danger btn-sm" :disabled="Boolean(testing[model.name]) || batchTesting" @click="removeModel(index, model)">删除</button></div>
                        </div>
                      </article>
                    </div>
                    <div v-if="!models.length" class="mt-3 rounded-lg border border-dashed border-line p-8 text-center text-sm text-soft">尚未添加模型</div>
                    <div v-if="testRecordRows.length" class="model-table-wrap mt-4">
                      <table class="table-eng min-w-[680px]" aria-label="模型测试记录表">
                        <thead><tr><th>模型</th><th class="w-24">结果</th><th class="w-28">协议</th><th>上游模型</th><th class="w-24 text-right">延迟</th><th>说明</th></tr></thead>
                        <tbody><tr v-for="row in testRecordRows" :key="row.model.name">
                          <td><code class="text-[12px]">{{ row.model.name }}</code></td><td><span v-if="testing[row.model.name] || row.result?.pending" class="chip chip-test">测试中</span><span v-else-if="row.result?.success" class="chip chip-run">通过</span><span v-else class="chip chip-trip">失败</span></td>
                          <td><code class="text-[12px]">{{ row.result?.protocol || '—' }}</code></td><td><code class="break-all text-[12px]">{{ row.result?.upstream || row.model.upstream || row.model.name }}</code></td><td class="num">{{ row.result?.latency_ms ? `${row.result.latency_ms} ms` : '—' }}</td><td class="max-w-md break-words text-[12px] text-soft">{{ row.result?.success ? (row.result.reply || '连通正常') : (row.result?.error || '等待试验结果') }}</td>
                        </tr></tbody>
                      </table>
                    </div>
                  </div>
                </ConsoleSection>
              </div>

              <div v-show="editorTab === 'overrides'" class="space-y-3">
                <ConsoleSection title="协议路由规则" description="按模型名称匹配目标协议；优先级低于模型显式协议配置。" eyebrow="Protocol rules">
                  <div class="space-y-2">
                    <div v-for="(rule, index) in rules" :key="index" class="rule-row">
                      <input v-model="rule.pattern" class="input input-mono text-[12px]" placeholder="^claude" :aria-label="`第 ${index + 1} 条协议规则正则`" />
                      <select v-model="rule.protocol" class="input text-[12px]"><option v-for="protocol in protocols" :key="protocol.value" :value="protocol.value">{{ protocol.name }}</option></select>
                      <button type="button" class="btn btn-danger btn-sm" @click="rules.splice(index, 1)"><ConsoleIcon name="trash" class="h-4 w-4" />删除</button>
                    </div>
                    <button type="button" class="btn btn-sm" @click="rules.push({ pattern: '', protocol: 'anthropic' })"><ConsoleIcon name="plus" class="h-4 w-4" />添加规则</button>
                  </div>
                </ConsoleSection>
                <ConsoleSection title="请求内容改写" description="在协议转换后、发送到上游前应用。" eyebrow="Overrides">
                  <div class="grid min-w-0 gap-6 xl:grid-cols-2">
                    <HeaderOverrideEditor v-model="form.header_override" :disabled="editorBusy" @validation="headerValidation = $event" />
                    <BodyOverrideEditor v-model="form.body_override" :disabled="editorBusy" @validation="bodyValidation = $event" />
                  </div>
                </ConsoleSection>
              </div>

              <div v-show="editorTab === 'reliability'" class="space-y-3">
                <ConsoleSection title="路由参与与负载" description="控制渠道是否参与路由，以及同优先级下的负载权重。" eyebrow="Reliability">
                  <div class="grid gap-4 md:grid-cols-2">
                    <div class="rounded-lg border border-line bg-surface p-3"><div class="flex items-center justify-between gap-3"><div><div class="text-sm font-semibold text-ink">渠道状态</div><p class="mt-1 text-xs text-soft">{{ form.status === 1 ? '当前参与模型路由。' : '当前不会接收新请求。' }}</p></div><button type="button" class="channel-switch" :class="{ 'channel-switch-on': form.status === 1 }" :aria-pressed="form.status === 1" @click="form.status = form.status === 1 ? 0 : 1"><span></span></button></div></div>
                    <div><label class="field-label" for="channel-weight">渠道权重</label><input id="channel-weight" v-model.number="form.weight" type="number" min="1" class="input input-mono" placeholder="1" /><p class="mt-1 text-[11px] text-soft">优先级仍由左侧队列拖拽顺序决定。</p></div>
                    <div class="md:col-span-2"><label class="field-label" for="channel-test-prompt">测试提示词覆盖</label><textarea id="channel-test-prompt" v-model="form.test_prompt" class="input min-h-24 resize-y" maxlength="4000" :placeholder="`留空继承全局：${globalTestPrompt}`"></textarea><p class="mt-1 text-[11px] text-soft">单测与批量体检优先使用此内容；留空时继承全局默认。</p></div>
                  </div>
                </ConsoleSection>
                <ConsoleSection title="健康检查与熔断" description="对已保存渠道运行全模型检查，或清除累计健康状态。" eyebrow="Health">
                  <template #actions><span v-if="selectedChannel" class="chip" :class="healthClass(channelHealth(selectedChannel))">{{ healthText(channelHealth(selectedChannel)) }}</span></template>
                  <InlineNotice v-if="!form.id" tone="info" title="保存后可用">创建渠道后即可运行全模型检查和重置健康状态。</InlineNotice>
                  <div v-else class="grid gap-3 sm:grid-cols-2">
                    <button type="button" class="btn justify-start" :disabled="checkupLoadingId !== null" @click="checkupChannel(selectedChannel || form)"><ConsoleIcon name="bolt" class="h-4 w-4" />{{ checkupLoadingId === form.id ? '检查中' : '运行全模型检查' }}</button>
                    <button type="button" class="btn justify-start" :disabled="resettingIds.has(form.id)" @click="resetBreaker(selectedChannel || form)"><ConsoleIcon name="arrowPath" class="h-4 w-4" />{{ resettingIds.has(form.id) ? '重置中' : breakerState(selectedChannel || form) === 'trip' ? '解除熔断' : '重置健康状态' }}</button>
                  </div>
                </ConsoleSection>
              </div>
            </main>
          </div>

          <footer class="channel-actionbar">
            <div class="min-w-0"><div class="flex items-center gap-2 text-xs font-semibold" :class="`save-state-${saveStatus.tone}`"><span class="save-state-dot"></span>{{ saveStatus.label }}</div><p class="mt-1 truncate text-[10px] text-soft">{{ saveHint }}</p></div>
            <div class="channel-action-buttons">
              <button v-if="form.id" type="button" class="btn btn-danger" :disabled="editorBusy || deletingIds.has(form.id)" @click="removeChannel(selectedChannel || form)"><ConsoleIcon name="trash" class="h-4 w-4" />{{ deletingIds.has(form.id) ? '删除中' : '删除' }}</button>
              <button type="button" class="btn" :disabled="editorBusy || (!isDirty && Boolean(form.id))" @click="closeEditor">取消</button>
              <button type="button" class="btn btn-primary" :disabled="editorBusy || !canSave || (!isDirty && Boolean(form.id))" @click="save"><ConsoleIcon name="checkCircle" class="h-4 w-4" />{{ saving ? '保存中' : form.id ? '保存更改' : '创建渠道' }}</button>
            </div>
          </footer>
        </div>
      </component>
    </div>

    <Modal :open="showCheckup" :title="`渠道检查记录 · ${checkupChannelName}`" width="max-w-4xl" @close="showCheckup = false">
      <div class="space-y-3">
        <div v-if="checkupSummary" class="flex flex-wrap items-center gap-2"><span class="chip chip-run">通过 {{ checkupSummary.success }}</span><span class="chip chip-trip">失败 {{ checkupSummary.failed }}</span><span class="chip chip-test">总计 {{ checkupSummary.total }}</span><span class="chip">合格率 {{ checkupRate }}%</span></div>
        <div v-if="checkupResults.length" class="space-y-2">
          <article v-for="(result, index) in checkupResults" :key="`${result.model}-${index}`" class="rounded-lg border border-line p-3">
            <div class="flex flex-wrap items-center justify-between gap-2"><code class="break-all text-[12px]">{{ result.model }}</code><span class="chip" :class="result.success ? 'chip-run' : 'chip-trip'">{{ result.success ? '通过' : '失败' }}</span></div>
            <div class="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-soft"><span>协议：<code>{{ result.protocol || '—' }}</code></span><span>上游：<code>{{ result.upstream || result.model }}</code></span><span>延迟：<code>{{ result.success && result.latency_ms ? `${result.latency_ms} ms` : '—' }}</code></span></div>
            <p class="mt-2 break-words text-xs leading-5 text-soft">{{ result.success ? (result.reply || '连通正常') : (result.error || '未返回错误说明') }}</p>
          </article>
        </div>
        <div v-else class="rounded-lg border border-dashed border-line px-4 py-8 text-center text-sm text-soft">无检查记录</div>
      </div>
      <template #footer><button type="button" class="btn" @click="showCheckup = false">关闭记录</button></template>
    </Modal>
  </div>
</template>

<style scoped>
.channels-workspace { display: grid; grid-template-columns: minmax(320px, 380px) minmax(0, 1fr); gap: 14px; min-width: 0; }
.channel-master, .channel-detail-host { height: calc(100dvh - 184px); min-height: 620px; }
.channel-master { display: flex; min-width: 0; flex-direction: column; }
.channel-list-scroll { min-height: 0; flex: 1; overflow-y: auto; overscroll-behavior: contain; }
.channel-list { display: grid; gap: 6px; background: rgb(var(--color-canvas)); }
.channel-row { border-color: transparent; background: rgb(var(--color-surface-1)); cursor: pointer; animation: channel-row-in 260ms both cubic-bezier(.2,.8,.2,1); animation-delay: min(calc(var(--row-index) * 22ms), 180ms); transition: border-color 150ms ease, box-shadow 150ms ease, opacity 150ms ease, transform 150ms ease; }
.channel-row:hover { border-color: rgb(var(--color-border)); transform: translateY(-1px); box-shadow: 0 7px 18px rgba(15, 23, 42, .06); }
.channel-row-selected { border-color: rgb(var(--color-accent)); background: rgb(var(--color-accent-muted)); box-shadow: inset 3px 0 0 rgb(var(--color-accent)); }
.channel-row-off { opacity: .68; }
.channel-row-dragging { opacity: .45; cursor: grabbing; }
.channel-row-dropzone { border-color: #a4382f; background: #f8ece8; }
.channel-drop-line { position: absolute; inset-inline: 6px; top: -4px; height: 3px; border-radius: 999px; background: #a4382f; }
.channel-grip { display: inline-flex; height: 26px; width: 24px; align-items: center; justify-content: center; border-radius: 5px; color: rgb(var(--color-text-muted)); cursor: grab; }
.channel-grip:hover:not(:disabled) { background: rgb(var(--color-surface-2)); color: rgb(var(--color-text)); }
.channel-grip:disabled { cursor: not-allowed; opacity: .35; }
.channel-state-dot { position: relative; display: inline-flex; width: 9px; height: 9px; flex: 0 0 auto; border-radius: 999px; background: #938a7c; }
.channel-state-dot i { position: absolute; inset: 3px; border-radius: inherit; background: white; opacity: .7; }
.channel-state-run { background: #50705a; box-shadow: 0 0 0 3px rgba(80,112,90,.12); }
.channel-state-test { background: #9a6a2f; box-shadow: 0 0 0 3px rgba(154,106,47,.12); }
.channel-state-trip { background: #a4382f; box-shadow: 0 0 0 3px rgba(164,56,47,.12); }
.channel-state-off { background: #938a7c; }
.channel-detail-empty { display: flex; min-width: 0; align-items: center; justify-content: center; padding: 28px; }
.channel-detail-host { min-width: 0; overflow: hidden; }
.channel-detail { display: flex; height: 100%; min-height: 0; flex-direction: column; background: rgb(var(--color-canvas)); }
.channel-detail-heading { display: flex; min-width: 0; flex: 0 0 auto; align-items: center; justify-content: space-between; gap: 16px; border-bottom: 1px solid rgb(var(--color-border)); background: rgb(var(--color-surface-1)); padding: 13px 16px; }
.channel-detail-layout { display: grid; min-height: 0; flex: 1; grid-template-columns: 168px minmax(0, 1fr); }
.channel-detail-nav { min-height: 0; overflow-y: auto; border-right: 1px solid rgb(var(--color-border)); background: rgb(var(--color-surface-1)); padding: 0 8px 12px; }
.channel-detail-nav nav button { display: grid; width: 100%; grid-template-columns: auto minmax(0,1fr) auto; align-items: center; gap: 8px; border-radius: 6px; padding: 9px 8px; text-align: left; color: rgb(var(--color-text-secondary)); transition: background-color 140ms ease, color 140ms ease; }
.channel-detail-nav nav button:hover { background: rgb(var(--color-surface-2)); color: rgb(var(--color-text)); }
.channel-detail-nav nav button[aria-selected='true'] { background: rgb(var(--color-accent-muted)); color: rgb(var(--color-accent-strong)); }
.channel-detail-nav b { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 11px; font-weight: 650; }
.channel-detail-nav small { display: block; overflow: hidden; margin-top: 2px; text-overflow: ellipsis; white-space: nowrap; font-size: 9px; color: rgb(var(--color-text-muted)); }
.detail-nav-state { width: 6px; height: 6px; border-radius: 999px; background: rgb(var(--color-text-muted)); }
.detail-nav-state.is-done { background: #50705a; }
.detail-nav-state.is-pending { background: #9a6a2f; }
.detail-summary { display: grid; gap: 6px; margin: 18px 8px 0; border-top: 1px solid rgb(var(--color-border)); padding-top: 12px; }
.detail-summary div { display: flex; min-width: 0; justify-content: space-between; gap: 8px; font-size: 9px; color: rgb(var(--color-text-muted)); }
.detail-summary dd { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-family: 'Spline Sans Mono', monospace; color: rgb(var(--color-text-secondary)); }
.channel-detail-content { min-width: 0; min-height: 0; overflow-y: auto; overscroll-behavior: contain; padding: 14px; }
.detail-mobile-nav { display: none; }
.channel-key-row { display: flex; min-width: 0; gap: 8px; }
.model-table-wrap { max-width: 100%; overflow-x: auto; border: 1px solid rgb(var(--color-border)); }
.rule-row { display: grid; min-width: 0; grid-template-columns: minmax(0,1fr) 140px auto; gap: 8px; }
.channel-actionbar { display: flex; min-width: 0; flex: 0 0 auto; align-items: center; justify-content: space-between; gap: 12px; border-top: 1px solid rgb(var(--color-border)); background: rgb(var(--color-surface-1)); padding: 10px 14px; box-shadow: 0 -8px 20px rgba(15,23,42,.04); }
.channel-action-buttons { display: flex; flex: 0 0 auto; align-items: center; gap: 8px; }
.save-state-dot { width: 7px; height: 7px; border-radius: 999px; background: currentColor; }
.save-state-saving { color: rgb(var(--color-warning)); }.save-state-dirty { color: rgb(var(--color-danger)); }.save-state-saved { color: rgb(var(--color-success)); }.save-state-idle { color: rgb(var(--color-text-secondary)); }
@keyframes channel-row-in { from { opacity: 0; transform: translateY(5px); } to { opacity: 1; transform: translateY(0); } }
@media (max-width: 1100px) and (min-width: 901px) {
  .channels-workspace { grid-template-columns: minmax(310px, 360px) minmax(0, 1fr); gap: 10px; }
  .channel-detail-layout { grid-template-columns: 148px minmax(0,1fr); }
  .channel-detail-content { padding: 12px; }
  .channel-key-row { flex-wrap: wrap; }.channel-key-row .input { flex-basis: 100%; }
}
@media (max-width: 767px) {
  .channels-workspace { display: block; }.channel-master { height: auto; min-height: 0; }.channel-list-scroll { overflow: visible; }
  .channel-detail { min-height: calc(100dvh - 88px); }.channel-detail-heading { padding: 2px 0 12px; }
  .detail-mobile-nav { display: grid; grid-template-columns: repeat(4, minmax(0,1fr)); gap: 4px; border-bottom: 1px solid rgb(var(--color-border)); padding: 8px 0; }
  .detail-mobile-nav button { display: flex; min-width: 0; flex-direction: column; align-items: center; justify-content: center; gap: 4px; border-radius: 6px; padding: 7px 3px; color: rgb(var(--color-text-secondary)); font-size: 9px; font-weight: 600; }
  .detail-mobile-nav button[aria-selected='true'] { background: rgb(var(--color-accent-muted)); color: rgb(var(--color-accent-strong)); }
  .channel-detail-layout { display: block; flex: 1 0 auto; }.channel-detail-nav { display: none; }.channel-detail-content { overflow: visible; padding: 12px 0; }
  .channel-actionbar { position: sticky; bottom: -20px; z-index: 10; margin: 0 -4px -20px; flex-wrap: wrap; padding: 10px 4px 12px; }
  .channel-action-buttons { width: 100%; }.channel-action-buttons .btn { min-width: 0; flex: 1; padding-inline: 8px; }
}
@media (max-width: 520px) {
  .channel-key-row { display: grid; grid-template-columns: repeat(2, minmax(0,1fr)); }.channel-key-row .input { grid-column: 1 / -1; }
  .rule-row { grid-template-columns: minmax(0,1fr); }.rule-row .btn { width: 100%; }.channel-detail-heading .chip { display: none; }
}
@media (max-width: 390px) { .channel-row { padding: 10px; }.detail-mobile-nav button { font-size: 8px; }.channel-actionbar { gap: 8px; }.channel-action-buttons { gap: 5px; } }
@media (prefers-reduced-motion: reduce) { .channel-row { animation: none; transition: none; } }
</style>
