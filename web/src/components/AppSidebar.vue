<script setup>
import { sheets } from '../router'
import ServiceStatus from './ServiceStatus.vue'

defineProps({
  routeName: { type: String, default: '' },
  username: { type: String, default: '管理员' },
  online: { type: Boolean, default: null },
  loggingOut: { type: Boolean, default: false },
})
defineEmits(['logout'])
</script>

<template>
  <aside class="command-rail" aria-label="控制台导航">
    <RouterLink to="/dashboard" class="rail-brand" aria-label="APIRelay 总览">
      <span class="rail-brand-mark" aria-hidden="true"><i></i><i></i><i></i></span>
      <span class="rail-brand-copy">AR</span>
    </RouterLink>

    <nav class="rail-nav" aria-label="主要导航">
      <RouterLink
        v-for="item in sheets"
        :key="item.name"
        :to="item.path"
        class="rail-link"
        :class="{ 'rail-link-active': routeName === item.name }"
        :aria-current="routeName === item.name ? 'page' : undefined"
      >
        <span class="rail-link-icon"><svg viewBox="0 0 24 24" aria-hidden="true"><path :d="item.icon" /></svg></span>
        <span class="rail-link-label">{{ item.label }}</span>
        <span v-if="routeName === item.name" class="rail-link-signal" aria-hidden="true"></span>
      </RouterLink>
    </nav>

    <div class="rail-foot">
      <div class="rail-service" :title="online ? '服务在线' : '状态未知'"><ServiceStatus :online="online" compact /></div>
      <button class="rail-account" type="button" :disabled="loggingOut" :title="`${username} · 退出登录`" @click="$emit('logout')">
        <span>{{ username.slice(0, 1).toUpperCase() }}</span>
        <small>{{ loggingOut ? '…' : '退出' }}</small>
      </button>
    </div>
  </aside>
</template>
