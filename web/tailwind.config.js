/** @type {import('tailwindcss').Config} */
const v = (name) => `rgb(var(${name}) / <alpha-value>)`

export default {
  darkMode: 'class',
  content: ['./index.html', './src/**/*.{vue,js,ts}'],
  theme: {
    extend: {
      colors: {
        bg: v('--c-bg'),
        surface: v('--c-surface'),
        panel: v('--c-panel'),
        elevated: v('--c-elevated'),
        border: v('--c-border'),
        line: v('--c-line'),
        'line-strong': v('--c-line-strong'),

        text: v('--c-text'),
        'text-dim': v('--c-text-dim'),
        'text-muted': v('--c-text-muted'),
        t1: v('--c-t1'),
        t2: v('--c-t2'),
        t3: v('--c-t3'),

        primary: v('--c-primary'),
        accent: v('--c-accent'),
        success: v('--c-success'),
        warning: v('--c-warning'),
        danger: v('--c-danger'),
        signal: v('--c-signal'),
        online: v('--c-online'),
        warn: v('--c-warn'),
        down: v('--c-down'),
      },
      fontFamily: {
        sans: ['"IBM Plex Sans"', 'ui-sans-serif', 'system-ui', 'sans-serif'],
        mono: ['"IBM Plex Mono"', 'ui-monospace', 'SFMono-Regular', 'monospace'],
      },
      fontSize: {
        '2xs': ['0.625rem', { lineHeight: '0.75rem' }],
      },
      borderRadius: {
        DEFAULT: '8px',
        lg: '12px',
        xl: '16px',
      },
      boxShadow: {
        pop: '0 18px 50px rgb(0 0 0 / 0.42)',
        panel: '0 24px 80px rgb(0 0 0 / 0.28)',
        signal: '0 0 34px rgb(var(--c-signal) / 0.18)',
      },
      animation: {
        'fade-in': 'fadeIn 0.2s ease-out',
        'pulse-glow': 'pulseGlow 2s ease-in-out infinite',
      },
      keyframes: {
        fadeIn: {
          '0%': { opacity: '0', transform: 'translateY(4px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' },
        },
        pulseGlow: {
          '0%, 100%': { opacity: '1' },
          '50%': { opacity: '0.5' },
        },
      },
    },
  },
  plugins: [],
}
