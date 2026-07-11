import { fmtTime } from './api'

// 模型/渠道调用健康度展示辅助。
// 阈值来源于设置页的 model-health 配置（healthy/warning），默认 95/70。

export const DEFAULT_HEALTH_CONFIG = {
  recent_count: 100,
  window_hours: 24,
  healthy_threshold: 95,
  warning_threshold: 70,
}

export function healthTotal(health) {
  return Number(health?.total) || 0
}

export function hasHealth(health) {
  return healthTotal(health) > 0
}

export function healthPercent(health) {
  if (!hasHealth(health)) return 0
  return Math.round((Number(health.availability) || 0) * 10) / 10
}

export function healthText(health) {
  if (!hasHealth(health)) return '未调用'
  return `${healthPercent(health)}% · ${Number(health.success) || 0}/${healthTotal(health)}`
}

// 依据阈值返回 chip class；config 为 model-health 配置对象。
export function healthClass(health, config = DEFAULT_HEALTH_CONFIG) {
  if (!hasHealth(health)) return ''
  const percent = healthPercent(health)
  if (percent >= Number(config?.healthy_threshold ?? 95)) return 'chip-run'
  if (percent >= Number(config?.warning_threshold ?? 70)) return 'chip-test'
  return 'chip-trip'
}

export function healthTitle(health) {
  if (!hasHealth(health)) return '尚无真实调用日志'
  const parts = [`成功 ${Number(health.success) || 0}`, `失败 ${Number(health.failed) || 0}`]
  if (health.last_failure_at) parts.push(`最近失败 ${fmtTime(health.last_failure_at)}`)
  if (health.last_error) parts.push(health.last_error)
  return parts.join(' · ')
}
