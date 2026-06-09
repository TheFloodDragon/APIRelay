<template>
  <el-config-provider>
    <div class="app-shell">
      <aside class="sidebar">
        <RouterLink to="/dashboard" class="brand">
          <span class="brand-mark">AR</span>
          <span>
            <strong>APIRelay</strong>
            <small>中转管理台</small>
            <span class="brand-subtitle">Global Proxy Hub</span>
          </span>
        </RouterLink>

        <nav class="sidebar-nav">
          <RouterLink v-for="item in navItems" :key="item.path" :to="item.path">
            <span class="nav-icon">
              <el-icon><component :is="item.icon" /></el-icon>
            </span>
            <span class="nav-text">
              <span>{{ item.label }}</span>
              <small>{{ item.desc }}</small>
            </span>
          </RouterLink>
        </nav>

        <div class="sidebar-footer">
          <strong>多渠道路由核心</strong>
          <p>统一管理全局代理、渠道健康、模型覆盖与请求观测。</p>
          <div class="sidebar-mini-status">
            <span class="online-dot"></span>
            <span>控制台连接正常</span>
          </div>
        </div>
      </aside>

      <div class="workspace">
        <header class="topbar">
          <div class="topbar-title">
            <span class="topbar-page-icon">
              <el-icon><component :is="currentIcon" /></el-icon>
            </span>
            <div>
              <p class="topbar-kicker">Admin Console · 全局代理控制台</p>
              <h2>{{ currentTitle }}</h2>
            </div>
          </div>
          <div class="topbar-actions">
            <div class="admin-key-group">
              <span class="admin-key-label">Admin Key</span>
              <el-input
                v-model="adminKey"
                class="admin-key-input"
                placeholder="管理密钥"
                show-password
                @change="saveAdminKey"
              />
            </div>
            <el-tag type="success" effect="light" class="status-tag">
              <span class="online-dot"></span>
              在线
            </el-tag>
          </div>
        </header>

        <main class="main">
          <RouterView v-slot="{ Component, route: viewRoute }">
            <Transition name="page" mode="out-in">
              <div v-if="Component" :key="viewRoute.fullPath" class="page-view">
                <component :is="Component" />
              </div>
            </Transition>
          </RouterView>
        </main>
      </div>
    </div>
  </el-config-provider>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink, RouterView, useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Collection, Connection, DataLine, Files, Setting, Switch } from '@element-plus/icons-vue'

const route = useRoute()
const adminKey = ref(localStorage.getItem('apirelay_admin_key') || 'change-me-in-production')

const navItems = [
  { path: '/dashboard', label: '仪表盘', desc: '运行总览与趋势', icon: DataLine },
  { path: '/channels', label: '渠道管理', desc: '供应商与优先级', icon: Switch },
  { path: '/models', label: '模型列表', desc: '模型映射与状态', icon: Collection },
  { path: '/proxy', label: '代理管理', desc: '队列、重试与熔断', icon: Connection },
  { path: '/settings', label: '全局设置', desc: '模型测试与系统偏好', icon: Setting },
  { path: '/logs', label: '请求日志', desc: '可观测请求链路', icon: Files }
]

const currentNavItem = computed(() => navItems.find((item) => route.path.startsWith(item.path)))
const currentTitle = computed(() => currentNavItem.value?.label || '管理台')
const currentIcon = computed(() => currentNavItem.value?.icon || DataLine)

function saveAdminKey() {
  localStorage.setItem('apirelay_admin_key', adminKey.value)
  ElMessage.success('管理密钥已保存')
}
</script>
