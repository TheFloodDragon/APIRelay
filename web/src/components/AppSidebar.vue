<script setup>
import { sheets } from '../router'
import ConsoleIcon from './ConsoleIcon.vue'
import ServiceStatus from './ServiceStatus.vue'

defineProps({
  routeName: { type: String, default: '' },
  username: { type: String, default: '管理员' },
  online: { type: Boolean, default: null },
  loggingOut: { type: Boolean, default: false },
  mobile: { type: Boolean, default: false },
})
defineEmits(['logout'])
</script>

<template>
  <aside class="console-sidebar" :class="{ 'console-sidebar-mobile': mobile }" aria-label="控制台导航">
    <RouterLink to="/dashboard" class="sidebar-brand" aria-label="APIRelay 运行总览">
      <span class="sidebar-brand-mark"><ConsoleIcon name="command" class="h-5 w-5" /></span>
      <span class="sidebar-brand-copy">
        <strong>APIRelay</strong>
        <small>routing operations</small>
      </span>
    </RouterLink>

    <div class="sidebar-nav-label">工作区</div>
    <nav class="sidebar-navigation" aria-label="主要导航">
      <RouterLink
        v-for="item in sheets"
        :key="item.name"
        :to="item.path"
        class="sidebar-link"
        :class="{ 'sidebar-link-active': routeName === item.name }"
        :aria-current="routeName === item.name ? 'page' : undefined"
        :title="item.label"
      >
        <ConsoleIcon :name="item.icon" class="sidebar-link-icon" />
        <span class="sidebar-link-label">{{ item.shortLabel || item.label }}</span>
        <span v-if="routeName === item.name" class="sidebar-link-indicator" aria-hidden="true"></span>
      </RouterLink>
    </nav>

    <div class="sidebar-footer">
      <div class="sidebar-service">
        <ServiceStatus :online="online" :compact="!mobile" />
      </div>
      <div class="sidebar-account">
        <span class="sidebar-avatar">{{ username.slice(0, 1).toUpperCase() }}</span>
        <span class="sidebar-account-copy"><small>当前账户</small><strong>{{ username }}</strong></span>
        <button
          class="sidebar-logout"
          type="button"
          :disabled="loggingOut"
          :aria-label="loggingOut ? '正在退出登录' : '退出登录'"
          title="退出登录"
          @click="$emit('logout')"
        >
          <ConsoleIcon name="arrowRightStart" class="h-4 w-4" />
        </button>
      </div>
    </div>
  </aside>
</template>
