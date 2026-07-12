<script setup>
import StatusBadge from './StatusBadge.vue'

defineProps({ rows: { type: Array, default: () => [] } })
</script>

<template>
  <div v-if="rows.length" class="route-matrix">
    <div class="route-matrix-table">
      <div class="route-matrix-head" aria-hidden="true">
        <span>优先级</span>
        <span>渠道 / 分组</span>
        <span>协议</span>
        <span class="route-matrix-number">模型</span>
        <span>熔断 / 健康</span>
        <span>最近事件</span>
      </div>
      <article v-for="row in rows" :key="row.id" class="route-matrix-row">
        <span class="route-priority">P{{ row.priority }}</span>
        <div class="route-identity">
          <div class="route-name" :title="row.name">{{ row.name }}</div>
          <div class="route-group" :title="row.group">{{ row.group }}</div>
        </div>
        <span class="route-protocol" :title="row.protocol">{{ row.protocol }}</span>
        <span class="route-model-count">{{ row.modelCount }}</span>
        <div class="route-state">
          <StatusBadge :status="row.status" :label="row.statusLabel" />
          <span v-if="row.healthMeta" class="route-state-meta">{{ row.healthMeta }}</span>
        </div>
        <div class="route-event">
          <div class="route-event-primary" :title="row.recentPrimary">{{ row.recentPrimary }}</div>
          <div v-if="row.recentSecondary" class="route-event-secondary" :title="row.recentSecondary">{{ row.recentSecondary }}</div>
        </div>
      </article>
    </div>

    <div class="route-matrix-mobile">
      <article v-for="row in rows" :key="row.id" class="route-mobile-row">
        <div class="route-mobile-main">
          <span class="route-priority">P{{ row.priority }}</span>
          <div class="route-identity">
            <div class="route-name">{{ row.name }}</div>
            <div class="route-mobile-meta">
              <span>{{ row.group }}</span>
              <span>{{ row.protocol }}</span>
              <span>{{ row.modelCount }} 模型</span>
            </div>
          </div>
          <StatusBadge :status="row.status" :label="row.statusLabel" />
        </div>
        <div class="route-mobile-event">
          <span v-if="row.healthMeta" class="route-state-meta">{{ row.healthMeta }}</span>
          <p>{{ row.recentPrimary }}</p>
          <p v-if="row.recentSecondary" class="route-event-secondary">{{ row.recentSecondary }}</p>
        </div>
      </article>
    </div>
  </div>
  <div v-else class="route-empty">尚未配置渠道</div>
</template>

<style scoped>
.route-matrix { min-width: 0; }
.route-matrix-table { display: none; }
.route-matrix-head,
.route-matrix-row {
  grid-template-columns: 3.25rem minmax(7.5rem, 1fr) minmax(4.5rem, .65fr) 3rem minmax(7.5rem, .9fr) minmax(10rem, 1.35fr);
  gap: .6rem;
}
.route-matrix-head {
  align-items: center;
  padding: .55rem .8rem;
  border-bottom: 1px solid rgb(var(--color-border));
  background: rgb(var(--color-surface-2) / .78);
  color: rgb(var(--color-text-muted));
  font-family: 'Spline Sans Mono', monospace;
  font-size: .55rem;
  font-weight: 600;
  letter-spacing: .07em;
  text-transform: uppercase;
}
.route-matrix-row {
  align-items: center;
  min-width: 0;
  padding: .68rem .8rem;
  border-bottom: 1px solid rgb(var(--color-border));
  transition: background-color 150ms ease;
}
.route-matrix-row:last-child { border-bottom: 0; }
.route-matrix-row:hover { background: rgb(var(--color-overlay) / .42); }
.route-priority,
.route-protocol,
.route-model-count {
  color: rgb(var(--color-text-secondary));
  font-family: 'Spline Sans Mono', monospace;
  font-size: .65rem;
}
.route-priority { color: rgb(var(--color-accent-soft)); }
.route-identity,
.route-event,
.route-state { min-width: 0; }
.route-name {
  overflow: hidden;
  color: rgb(var(--color-text));
  font-size: .75rem;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.route-group,
.route-state-meta,
.route-event-secondary {
  overflow: hidden;
  color: rgb(var(--color-text-muted));
  font-size: .625rem;
  line-height: 1rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.route-group { margin-top: .1rem; }
.route-protocol { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.route-matrix-number,
.route-model-count { text-align: right; }
.route-model-count { color: rgb(var(--color-text)); }
.route-state {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: .2rem;
}
.route-event-primary {
  overflow: hidden;
  color: rgb(var(--color-text));
  font-size: .675rem;
  line-height: 1rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.route-matrix-mobile { display: block; }
.route-mobile-row {
  min-width: 0;
  padding: .8rem;
  border-bottom: 1px solid rgb(var(--color-border));
}
.route-mobile-row:last-child { border-bottom: 0; }
.route-mobile-main {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: start;
  gap: .65rem;
}
.route-mobile-meta {
  display: flex;
  min-width: 0;
  flex-wrap: wrap;
  gap: .15rem .65rem;
  margin-top: .2rem;
  color: rgb(var(--color-text-muted));
  font-size: .625rem;
}
.route-mobile-event {
  min-width: 0;
  margin-top: .6rem;
  padding-left: 2.35rem;
  color: rgb(var(--color-text-secondary));
  font-size: .675rem;
  line-height: 1.05rem;
}
.route-mobile-event p { overflow-wrap: anywhere; }
.route-mobile-event .route-state-meta { display: block; margin-bottom: .15rem; }
.route-mobile-event .route-event-secondary { margin-top: .15rem; white-space: normal; }
.route-empty {
  padding: 2.5rem 1rem;
  color: rgb(var(--color-text-muted));
  font-size: .75rem;
  text-align: center;
}

@media (min-width: 900px) {
  .route-matrix-table { display: block; }
  .route-matrix-head,
  .route-matrix-row { display: grid; }
  .route-matrix-mobile { display: none; }
}
</style>
