/** @type {import('tailwindcss').Config} */
//
// APIRelay 设计令牌 ——「信号路由控制台」
// 隐喻：工程仪表面板（示波器/频谱仪/机舱仪表）
// 配色策略：石墨冷灰基底 + 单一信号色（示波器青绿），状态语义色仅用于点与细条。
// 颜色通过 CSS 变量驱动（见 style.css），支持亮/暗双主题与 alpha 通道。
//
const v = (name) => `rgb(var(${name}) / <alpha-value>)`

export default {
  darkMode: 'class',
  content: ['./index.html', './src/**/*.{vue,js,ts}'],
  theme: {
    extend: {
      colors: {
        // 结构层
        surface: v('--c-surface'),     // 最底层背景（仪表面板黑 / 冷白）
        panel: v('--c-panel'),         // 面板/卡片
        'panel-2': v('--c-panel-2'),   // 次级面板（输入框、内嵌区）
        line: v('--c-line'),           // hairline 分隔线（主要分隔手段）
        'line-strong': v('--c-line-strong'),

        // 文本三级灰阶
        t1: v('--c-t1'),               // 主文
        t2: v('--c-t2'),               // 次文
        t3: v('--c-t3'),               // 弱文 / 刻度标签

        // 唯一强调：信号色（示波器青绿）
        signal: v('--c-signal'),
        'signal-soft': v('--c-signal-soft'),

        // 状态语义色（仅用于脉冲点 / 细条）
        online: v('--c-online'),       // 绿 在线
        warn: v('--c-warn'),           // 琥珀 降级/限流/冷却
        down: v('--c-down'),           // 红 故障/禁用
      },
      fontFamily: {
        // 数据 / 数字 / ID / 模型名 —— 等宽（签名所在）
        mono: ['"IBM Plex Mono"', 'ui-monospace', 'SFMono-Regular', 'Menlo', 'Consolas', 'monospace'],
        // 界面标题 / 正文 —— 工程感无衬线
        sans: ['"IBM Plex Sans"', 'Inter', 'system-ui', '-apple-system', 'Segoe UI', '"Microsoft YaHei"', 'sans-serif'],
      },
      fontSize: {
        // 明确的类型刻度
        '2xs': ['11px', { lineHeight: '14px' }],
        xs: ['12px', { lineHeight: '16px' }],
        sm: ['13px', { lineHeight: '18px' }],
        base: ['15px', { lineHeight: '22px' }],
        lg: ['20px', { lineHeight: '26px' }],
        xl: ['28px', { lineHeight: '32px' }],
      },
      borderRadius: {
        // 克制圆角：8px 为主
        DEFAULT: '6px',
        md: '6px',
        lg: '8px',
        xl: '10px',
      },
      boxShadow: {
        // 去重阴影，仅保留极轻浮层 / 弹窗
        panel: '0 1px 2px 0 rgb(0 0 0 / 0.04)',
        pop: '0 8px 28px -8px rgb(0 0 0 / 0.30)',
        'signal-focus': '0 0 0 2px rgb(var(--c-signal) / 0.35)',
      },
      keyframes: {
        'fade-in': {
          '0%': { opacity: '0' },
          '100%': { opacity: '1' },
        },
        'pop-in': {
          '0%': { opacity: '0', transform: 'translateY(6px) scale(0.99)' },
          '100%': { opacity: '1', transform: 'translateY(0) scale(1)' },
        },
        // 在线脉冲点：信号色向外扩散
        'signal-pulse': {
          '0%': { boxShadow: '0 0 0 0 rgb(var(--c-online) / 0.55)' },
          '70%': { boxShadow: '0 0 0 5px rgb(var(--c-online) / 0)' },
          '100%': { boxShadow: '0 0 0 0 rgb(var(--c-online) / 0)' },
        },
        // 开机序列：扫描线
        sweep: {
          '0%': { transform: 'translateX(-100%)' },
          '100%': { transform: 'translateX(100%)' },
        },
        'boot-line': {
          '0%': { transform: 'scaleX(0)', opacity: '0.2' },
          '100%': { transform: 'scaleX(1)', opacity: '1' },
        },
      },
      animation: {
        'fade-in': 'fade-in 0.2s ease-out',
        'pop-in': 'pop-in 0.18s cubic-bezier(0.16, 1, 0.3, 1)',
        'signal-pulse': 'signal-pulse 2s cubic-bezier(0.4, 0, 0.6, 1) infinite',
        sweep: 'sweep 1.6s linear infinite',
        'boot-line': 'boot-line 0.5s cubic-bezier(0.16, 1, 0.3, 1)',
      },
    },
  },
  plugins: [],
}
