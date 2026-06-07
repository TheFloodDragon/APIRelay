<template>
  <el-config-provider>
    <div class="app-shell">
      <aside class="sidebar">
        <RouterLink to="/dashboard" class="brand">
          <span class="brand-mark">AR</span>
          <span>
            <strong>APIRelay</strong>
            <small>中转管理台</small>
          </span>
        </RouterLink>

        <nav class="sidebar-nav">
          <RouterLink v-for="item in navItems" :key="item.path" :to="item.path">
            <el-icon><component :is="item.icon" /></el-icon>
            <span>{{ item.label }}</span>
          </RouterLink>
        </nav>

        <div class="sidebar-footer">
          <strong>聚合中转站</strong>
          <p>统一管理渠道、模型路由与请求观测数据。</p>
        </div>
      </aside>

      <div class="workspace">
        <header class="topbar">
          <div>
            <p class="topbar-kicker">Admin Console</p>
            <h2>{{ currentTitle }}</h2>
          </div>
          <div class="topbar-actions">
            <el-input
              v-model="adminKey"
              class="admin-key-input"
              placeholder="管理密钥"
              show-password
              @change="saveAdminKey"
            />
            <el-tag type="success" effect="light" class="status-tag">
              <span class="online-dot"></span>
              在线
            </el-tag>
          </div>
        </header>

        <main class="main">
          <RouterView v-slot="{ Component }">
            <Transition name="page" mode="out-in">
              <component :is="Component" />
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
import { Collection, DataLine, Files, Switch } from '@element-plus/icons-vue'

const route = useRoute()
const adminKey = ref(localStorage.getItem('apirelay_admin_key') || 'change-me-in-production')

const navItems = [
  { path: '/dashboard', label: '仪表盘', icon: DataLine },
  { path: '/channels', label: '渠道管理', icon: Switch },
  { path: '/models', label: '模型列表', icon: Collection },
  { path: '/logs', label: '请求日志', icon: Files }
]

const currentTitle = computed(() => navItems.find((item) => route.path.startsWith(item.path))?.label || '管理台')

function saveAdminKey() {
  localStorage.setItem('apirelay_admin_key', adminKey.value)
  ElMessage.success('管理密钥已保存')
}
</script>
