<script setup>
// PatchRoute —— 配线架路由图（全站签名元素）
// 左列：入站协议端口 · 中央：IR 中枢 · 右列：上游渠道端口
// 在线渠道用黄铜跳线，冷却/故障用暗线，附极克制的信号流动点。
import { computed } from 'vue'

const props = defineProps({
  channels: { type: Array, default: () => [] },
})

// 入站协议端口（固定三路）
const inbound = [
  { key: 'openai', label: 'OpenAI', code: 'IN·01' },
  { key: 'anthropic', label: 'Anthropic', code: 'IN·02' },
  { key: 'responses', label: 'Responses', code: 'IN·03' },
]

function stateOf(ch) {
  if (ch.status !== 1) return 'down'
  if (ch.cooldown_until && ch.cooldown_until > Date.now()) return 'warn'
  return 'online'
}

// 上游渠道端口：按优先级排序，最多展示 6 路
const MAX = 6
const sorted = computed(() =>
  [...props.channels].sort((a, b) => (a.priority - b.priority) || (a.id - b.id))
)
const upstream = computed(() =>
  sorted.value.slice(0, MAX).map((ch, i) => ({
    id: ch.id,
    name: ch.name,
    code: 'UP·' + String(i + 1).padStart(2, '0'),
    state: stateOf(ch),
  }))
)
const overflow = computed(() => Math.max(0, sorted.value.length - MAX))

// ===== SVG 几何 =====
const W = 760
const rowH = 46
const padY = 26
const H = computed(() => Math.max(inbound.length, upstream.value.length || 1) * rowH + padY * 2)

const leftX = 150      // 左端口跳线锚点（右边缘）
const hubLX = 330      // 中枢左边缘
const hubRX = 430      // 中枢右边缘
const rightX = 610     // 右端口跳线锚点（左边缘）
const hubCY = computed(() => H.value / 2)

function colYs(count) {
  const total = count * rowH
  const start = (H.value - total) / 2 + rowH / 2
  return Array.from({ length: count }, (_, i) => start + i * rowH)
}
const inY = computed(() => colYs(inbound.length))
const upY = computed(() => colYs(upstream.value.length || 1))

// 左跳线：入站端口 → 中枢
const leftCords = computed(() =>
  inbound.map((p, i) => {
    const y = inY.value[i]
    const cy = hubCY.value
    return { key: p.key, d: `M ${leftX} ${y} C ${leftX + 70} ${y}, ${hubLX - 70} ${cy}, ${hubLX} ${cy}` }
  })
)
// 右跳线：中枢 → 上游端口
const rightCords = computed(() =>
  upstream.value.map((p, i) => {
    const y = upY.value[i]
    const cy = hubCY.value
    return {
      key: p.id,
      state: p.state,
      d: `M ${hubRX} ${cy} C ${hubRX + 70} ${cy}, ${rightX - 70} ${y}, ${rightX} ${y}`,
    }
  })
)

const cordColor = { online: 'var(--brass)', warn: 'var(--amber)', down: 'var(--line-2)' }
const onlineCount = computed(() => upstream.value.filter(u => u.state === 'online').length)
</script>

<template>
  <div class="panel overflow-hidden">
    <div class="px-4 h-11 flex items-center justify-between border-b border-line">
      <div class="flex items-center gap-2">
        <span class="font-mono text-sm font-medium text-t1">路由配线架</span>
        <span class="tick">PATCHBAY</span>
      </div>
      <span class="font-mono text-2xs text-t3">{{ onlineCount }}/{{ upstream.length }} 跳线接通</span>
    </div>

    <div class="p-4">
      <div class="relative w-full">
        <svg :viewBox="`0 0 ${W} ${H}`" class="w-full h-auto" role="img" aria-label="配线架路由图" preserveAspectRatio="xMidYMid meet">
          <!-- 左跳线 -->
          <g fill="none" stroke-linecap="round">
            <path v-for="c in leftCords" :key="'l'+c.key" :d="c.d"
              :stroke="`rgb(${cordColor.online} / 0.55)`" stroke-width="2" />
          </g>
          <!-- 右跳线 -->
          <g fill="none" stroke-linecap="round">
            <template v-for="c in rightCords" :key="'r'+c.key">
              <path :d="c.d"
                :stroke="c.state === 'online' ? `rgb(${cordColor.online} / 0.85)` : c.state === 'warn' ? `rgb(${cordColor.warn} / 0.5)` : `rgb(${cordColor.down} / 0.55)`"
                :stroke-width="c.state === 'online' ? 2.4 : 1.6"
                :stroke-dasharray="c.state === 'down' ? '3 5' : 'none'" />
              <!-- 信号流动点：仅在线跳线，克制 -->
              <path v-if="c.state === 'online'" :d="c.d" fill="none"
                :stroke="`rgb(${cordColor.online})`" stroke-width="2.4" stroke-linecap="round"
                stroke-dasharray="2 26" class="motion-safe:animate-flow" style="opacity:0.9" />
            </template>
          </g>

          <!-- 中枢 IR 核 -->
          <g>
            <rect :x="hubLX" :y="hubCY - 34" :width="hubRX - hubLX" height="68" rx="12"
              fill="rgb(var(--panel-2))" :stroke="`rgb(${cordColor.online} / 0.45)`" stroke-width="1.5" />
            <text :x="(hubLX + hubRX) / 2" :y="hubCY - 6" text-anchor="middle"
              fill="rgb(var(--brass))" font-family="'IBM Plex Mono', monospace" font-size="16" font-weight="600">IR</text>
            <text :x="(hubLX + hubRX) / 2" :y="hubCY + 14" text-anchor="middle"
              fill="rgb(var(--t3))" font-family="'IBM Plex Mono', monospace" font-size="9" letter-spacing="1">中枢</text>
          </g>

          <!-- 左端口插孔 -->
          <g v-for="(p, i) in inbound" :key="'ip'+p.key">
            <circle :cx="leftX" :cy="inY[i]" r="5" fill="rgb(var(--ink))" :stroke="`rgb(${cordColor.online})`" stroke-width="2" />
          </g>
          <!-- 右端口插孔 -->
          <g v-for="(p, i) in upstream" :key="'up'+p.id">
            <circle :cx="rightX" :cy="upY[i]" r="5" fill="rgb(var(--ink))"
              :stroke="p.state === 'online' ? `rgb(${cordColor.online})` : p.state === 'warn' ? `rgb(${cordColor.warn})` : `rgb(${cordColor.down})`"
              stroke-width="2" />
          </g>
        </svg>

        <!-- 端口标签叠加层（HTML，保证可读与响应式截断） -->
        <div class="pointer-events-none absolute inset-0 flex justify-between">
          <!-- 左列：入站协议 -->
          <div class="flex flex-col justify-center gap-0 w-[19%]">
            <div v-for="p in inbound" :key="p.key"
              class="flex items-center" :style="{ height: (100 / inbound.length) + '%' }">
              <div class="min-w-0">
                <div class="font-mono text-2xs text-t3 leading-none">{{ p.code }}</div>
                <div class="text-xs font-medium text-t1 truncate">{{ p.label }}</div>
              </div>
            </div>
          </div>
          <!-- 右列：上游渠道 -->
          <div class="flex flex-col justify-center items-end gap-0 w-[22%]">
            <div v-for="p in upstream" :key="p.id"
              class="flex items-center justify-end w-full" :style="{ height: (100 / (upstream.length || 1)) + '%' }">
              <div class="min-w-0 text-right">
                <div class="font-mono text-2xs text-t3 leading-none">{{ p.code }}</div>
                <div class="text-xs font-medium truncate"
                  :class="p.state === 'online' ? 'text-t1' : p.state === 'warn' ? 'text-amber' : 'text-t3'">{{ p.name }}</div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div v-if="overflow" class="mt-1 text-center font-mono text-2xs text-t3">+{{ overflow }} 路上游未在图中展示</div>
      <div v-if="!upstream.length" class="empty-state !py-6">
        <span class="font-mono text-2xs">暂无上游渠道，先在「渠道」中添加</span>
      </div>
    </div>
  </div>
</template>
