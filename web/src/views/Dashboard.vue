<script setup>
import { ref, onMounted } from 'vue'
import api from '../api'

const data = ref({ channel_count: 0, stat: {} })
onMounted(async () => {
  data.value = await api.get('/dashboard')
})
</script>

<template>
  <div>
    <h2 class="text-lg font-semibold mb-4">仪表盘</h2>
    <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
      <div class="bg-white rounded-lg shadow p-4">
        <div class="text-xs text-slate-500">渠道数</div>
        <div class="text-2xl font-semibold mt-1">{{ data.channel_count }}</div>
      </div>
      <div class="bg-white rounded-lg shadow p-4">
        <div class="text-xs text-slate-500">总请求数</div>
        <div class="text-2xl font-semibold mt-1">{{ data.stat?.total_requests || 0 }}</div>
      </div>
      <div class="bg-white rounded-lg shadow p-4">
        <div class="text-xs text-slate-500">Prompt Tokens</div>
        <div class="text-2xl font-semibold mt-1">{{ data.stat?.total_prompt_tokens || 0 }}</div>
      </div>
      <div class="bg-white rounded-lg shadow p-4">
        <div class="text-xs text-slate-500">Completion Tokens</div>
        <div class="text-2xl font-semibold mt-1">{{ data.stat?.total_completion_tokens || 0 }}</div>
      </div>
    </div>
  </div>
</template>
