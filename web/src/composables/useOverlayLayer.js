import { nextTick, onBeforeUnmount, watch } from 'vue'

// 全局弹层栈。Modal 与 Drawer 都在捕获阶段监听 document 的 keydown，
// 而捕获阶段回调按注册顺序触发 —— 先打开的弹层会先收到 Esc 并 stopPropagation。
// 结果「抽屉里再弹确认框」时按 Esc 关掉的是抽屉而不是确认框。
//
// 这里维护一个模块级栈：只有栈顶实例响应 Esc，其余直接忽略。
const stack = []

// body 滚动锁：多个弹层同时打开时只有第一个负责保存/恢复 overflow，
// 否则内层弹层关闭时会把 overflow 恢复成 'hidden' 之外的值，导致外层弹层背景可滚动。
let bodyLockCount = 0
let savedOverflow = ''

function lockBody() {
  if (bodyLockCount === 0) {
    savedOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
  }
  bodyLockCount += 1
}

function unlockBody() {
  if (bodyLockCount === 0) return
  bodyLockCount -= 1
  if (bodyLockCount === 0) {
    document.body.style.overflow = savedOverflow
    savedOverflow = ''
  }
}

const FOCUS_SELECTOR = [
  'button:not(:disabled)',
  '[href]',
  'input:not(:disabled)',
  'select:not(:disabled)',
  'textarea:not(:disabled)',
  '[tabindex]:not([tabindex="-1"])',
].join(', ')

/**
 * 为一个模态层（Modal / Drawer）接管 Esc 关闭、Tab 焦点陷阱、
 * 焦点恢复与 body 滚动锁。
 *
 * @param {object} options
 * @param {import('vue').Ref<HTMLElement|null>} options.panel 面板根元素
 * @param {() => boolean} options.isOpen 当前是否打开
 * @param {() => boolean} options.isPersistent 是否禁止 Esc / 点击遮罩关闭
 * @param {() => void} options.onRequestClose 请求关闭时的回调
 */
export function useOverlayLayer({ panel, isOpen, isPersistent, onRequestClose }) {
  // 用一个稳定的令牌标识本实例在栈中的身份。
  const token = {}
  let previousFocus = null
  let locked = false

  function isTopOfStack() {
    return stack.length > 0 && stack[stack.length - 1] === token
  }

  function handleKeydown(event) {
    // 只有栈顶弹层处理按键：嵌套场景下内层先关闭，符合用户预期。
    if (!isTopOfStack()) return

    if (event.key === 'Escape') {
      if (!isPersistent()) {
        event.stopPropagation()
        onRequestClose()
      }
      return
    }
    if (event.key !== 'Tab' || !panel.value) return

    const focusable = [...panel.value.querySelectorAll(FOCUS_SELECTOR)]
      .filter((element) => element.offsetParent !== null)
    if (!focusable.length) {
      event.preventDefault()
      panel.value.focus()
      return
    }
    const first = focusable[0]
    const last = focusable[focusable.length - 1]
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault()
      last.focus()
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault()
      first.focus()
    }
  }

  function teardown({ restoreFocus }) {
    const index = stack.indexOf(token)
    if (index !== -1) stack.splice(index, 1)
    document.removeEventListener('keydown', handleKeydown, true)
    if (locked) {
      unlockBody()
      locked = false
    }
    if (restoreFocus) previousFocus?.focus?.()
    previousFocus = null
  }

  watch(
    isOpen,
    async (open) => {
      if (open) {
        if (stack.includes(token)) return
        previousFocus = document.activeElement
        stack.push(token)
        lockBody()
        locked = true
        document.addEventListener('keydown', handleKeydown, true)
        await nextTick()
        const target = panel.value?.querySelector('[data-autofocus]')
          || panel.value?.querySelector(FOCUS_SELECTOR)
        ;(target || panel.value)?.focus()
      } else {
        teardown({ restoreFocus: true })
      }
    },
    { immediate: true }
  )

  // 卸载时不恢复焦点：此时目标元素往往也已随路由切换消失，
  // 强行 focus 会把焦点跳到不相关的位置。
  onBeforeUnmount(() => teardown({ restoreFocus: false }))
}

// 供测试断言栈是否被正确清理。
export function overlayStackDepth() {
  return stack.length
}
