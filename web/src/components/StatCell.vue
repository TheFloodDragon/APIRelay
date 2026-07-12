<script setup>
import ConsoleIcon from './ConsoleIcon.vue'

defineProps({
  label: { type: String, required: true },
  value: { type: [String, Number], default: '—' },
  unit: { type: String, default: '' },
  hint: { type: String, default: '' },
  icon: { type: String, default: '' },
  tone: { type: String, default: 'neutral' },
})
</script>

<template>
  <div class="stat-cell" :class="`stat-cell-${tone}`" :title="hint">
    <div class="stat-cell-head">
      <span class="stat-cell-label">{{ label }}</span>
      <ConsoleIcon v-if="icon" :name="icon" class="stat-cell-icon" />
    </div>
    <div class="stat-cell-reading">
      <span class="stat-cell-value">{{ value }}</span>
      <span v-if="unit" class="stat-cell-unit">{{ unit }}</span>
    </div>
    <p v-if="hint" class="stat-cell-hint">{{ hint }}</p>
  </div>
</template>

<style scoped>
.stat-cell {
  position: relative;
  min-width: 0;
  padding: .8rem .9rem .75rem;
  border-right: 1px solid rgb(var(--color-border));
  background: rgb(var(--color-surface-1));
}
.stat-cell::before {
  content: '';
  position: absolute;
  inset: 0 auto 0 0;
  width: 2px;
  background: rgb(var(--color-accent) / .72);
}
.stat-cell-success::before { background: rgb(var(--color-success)); }
.stat-cell-warning::before { background: rgb(var(--color-warning)); }
.stat-cell-danger::before { background: rgb(var(--color-danger)); }
.stat-cell-head {
  display: flex;
  min-width: 0;
  align-items: center;
  justify-content: space-between;
  gap: .5rem;
}
.stat-cell-label {
  overflow: hidden;
  color: rgb(var(--color-text-secondary));
  font-size: .625rem;
  font-weight: 600;
  letter-spacing: .08em;
  text-overflow: ellipsis;
  text-transform: uppercase;
  white-space: nowrap;
}
.stat-cell-icon {
  width: .9rem;
  height: .9rem;
  flex: 0 0 auto;
  color: rgb(var(--color-text-muted));
}
.stat-cell-reading {
  display: flex;
  min-width: 0;
  align-items: baseline;
  gap: .35rem;
  margin-top: .45rem;
}
.stat-cell-value {
  min-width: 0;
  overflow: hidden;
  color: rgb(var(--color-text));
  font-family: 'Spline Sans Mono', monospace;
  font-size: clamp(1.05rem, 2.4vw, 1.35rem);
  font-weight: 600;
  line-height: 1.15;
  letter-spacing: -.04em;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.stat-cell-unit {
  color: rgb(var(--color-text-muted));
  font-family: 'Spline Sans Mono', monospace;
  font-size: .55rem;
  letter-spacing: .08em;
  text-transform: uppercase;
}
.stat-cell-hint {
  min-width: 0;
  overflow: hidden;
  margin-top: .35rem;
  color: rgb(var(--color-text-muted));
  font-size: .625rem;
  line-height: 1rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

@media (max-width: 639px) {
  .stat-cell:nth-child(2n) { border-right: 0; }
  .stat-cell:nth-child(-n + 2) { border-bottom: 1px solid rgb(var(--color-border)); }
}

@media (min-width: 640px) {
  .stat-cell:last-child { border-right: 0; }
}
</style>
