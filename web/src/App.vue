<template>
  <el-config-provider>
    <div class="app-shell">
      <aside class="sidebar">
        <div class="brand">APIRelay</div>
        <nav>
          <RouterLink to="/dashboard">仪表盘</RouterLink>
          <RouterLink to="/channels">渠道管理</RouterLink>
        </nav>
      </aside>
      <main class="main">
        <RouterView />
      </main>
    </div>
  </el-config-provider>
</template>
