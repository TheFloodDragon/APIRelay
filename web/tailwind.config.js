/** @type {import('tailwindcss').Config} */
const v = (name) => `rgb(var(${name}) / <alpha-value>)`

export default {
  darkMode: 'class',
  content: ['./index.html', './src/**/*.{vue,js,ts}'],
  theme: {
    extend: {
      colors: {
        // 结构：暖炭底 + 抬升面
        ink: v('--ink'),
        bg: v('--ink'),
        surface: v('--panel'),
        panel: v('--panel'),
        'panel-2': v('--panel-2'),
        elevated: v('--panel-2'),
        border: v('--line'),
        line: v('--line'),
        'line-2': v('--line-2'),
        'line-strong': v('--line-2'),

        // 文本：暖白三级递减
        text: v('--t1'),
        'text-dim': v('--t2'),
        'text-muted': v('--t3'),
        t1: v('--t1'),
        t2: v('--t2'),
        t3: v('--t3'),

        // 信号语义
        brass: v('--brass'),
        electric: v('--electric'),
        jade: v('--jade'),
        amber: v('--amber'),
        rust: v('--rust'),

        // 兼容旧语义别名 → 映射到新体系
        primary: v('--brass'),
        accent: v('--electric'),
        signal: v('--brass'),
        online: v('--jade'),
        warn: v('--amber'),
        down: v('--rust'),
        success: v('--jade'),
        warning: v('--amber'),
        danger: v('--rust'),
      },
      fontFamily: {
        sans: ['"IBM Plex Sans"', 'ui-sans-serif', 'system-ui', 'sans-serif'],
        mono: ['"IBM Plex Mono"', 'ui-monospace', 'SFMono-Regular', 'monospace'],
      },
      fontSize: {
        '2xs': ['0.625rem', { lineHeight: '0.85rem' }],
      },
      borderRadius: {
        DEFAULT: '6px',
        lg: '9px',
        xl: '13px',
        '2xl': '18px',
      },
      boxShadow: {
        pop: '0 16px 44px rgb(0 0 0 / 0.5)',
        panel: '0 20px 60px rgb(0 0 0 / 0.38)',
        brass: '0 0 24px rgb(var(--brass) / 0.22)',
        electric: '0 0 22px rgb(var(--electric) / 0.28)',
        inset: 'inset 0 1px 0 rgb(255 255 255 / 0.03)',
      },
      animation: {
        'fade-in': 'fadeIn 0.2s ease-out',
        'pulse-glow': 'pulseGlow 2.4s ease-in-out infinite',
        'flow': 'flow 2.6s linear infinite',
      },
      keyframes: {
        fadeIn: {
          '0%': { opacity: '0', transform: 'translateY(4px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' },
        },
        pulseGlow: {
          '0%, 100%': { opacity: '1' },
          '50%': { opacity: '0.45' },
        },
        flow: {
          '0%': { strokeDashoffset: '28' },
          '100%': { strokeDashoffset: '0' },
        },
      },
    },
  },
  plugins: [],
}
