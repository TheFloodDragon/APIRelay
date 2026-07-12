/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{vue,js}'],
  theme: {
    colors: {
      transparent: 'transparent',
      current: 'currentColor',
      white: '#FFFFFF',
      black: '#000000',
      canvas: 'rgb(var(--color-canvas) / <alpha-value>)',
      paper: 'rgb(var(--color-surface-1) / <alpha-value>)',
      surface: 'rgb(var(--color-surface-2) / <alpha-value>)',
      panel: 'rgb(var(--color-surface-3) / <alpha-value>)',
      sidebar: 'rgb(var(--color-sidebar) / <alpha-value>)',
      ink: 'rgb(var(--color-text) / <alpha-value>)',
      soft: 'rgb(var(--color-text-secondary) / <alpha-value>)',
      faint: 'rgb(var(--color-text-muted) / <alpha-value>)',
      line: 'rgb(var(--color-border) / <alpha-value>)',
      ghost: 'rgb(var(--color-overlay) / <alpha-value>)',
      blue: {
        DEFAULT: 'rgb(var(--color-accent) / <alpha-value>)',
        deep: 'rgb(var(--color-accent-strong) / <alpha-value>)',
        wash: 'rgb(var(--color-accent-muted) / <alpha-value>)',
        grid: 'rgb(var(--color-accent-soft) / <alpha-value>)',
      },
      run: {
        DEFAULT: 'rgb(var(--color-success) / <alpha-value>)',
        wash: 'rgb(var(--color-success-muted) / <alpha-value>)',
      },
      test: {
        DEFAULT: 'rgb(var(--color-warning) / <alpha-value>)',
        wash: 'rgb(var(--color-warning-muted) / <alpha-value>)',
      },
      trip: {
        DEFAULT: 'rgb(var(--color-danger) / <alpha-value>)',
        wash: 'rgb(var(--color-danger-muted) / <alpha-value>)',
      },
    },
    fontFamily: {
      sans: ['Segoe UI Variable', 'Segoe UI', 'PingFang SC', 'Microsoft YaHei', 'system-ui', 'sans-serif'],
      cond: ['Saira SemiCondensed', 'Segoe UI Variable', 'PingFang SC', 'Microsoft YaHei', 'sans-serif'],
      mono: ['Spline Sans Mono', 'ui-monospace', 'SFMono-Regular', 'Consolas', 'monospace'],
    },
    extend: {
      boxShadow: {
        sheet: '0 1px 2px rgba(0, 0, 0, .24), 0 16px 36px rgba(0, 0, 0, .16)',
        lift: '0 22px 70px rgba(0, 0, 0, .42)',
        insetline: 'inset 0 0 0 1px rgb(var(--color-border) / .85)',
      },
      fontSize: {
        '2xs': ['0.6875rem', { lineHeight: '1rem' }],
      },
      borderRadius: {
        DEFAULT: '0.375rem',
      },
      zIndex: {
        toast: '90',
        overlay: '80',
        sidebar: '50',
        topbar: '40',
      },
    },
  },
  plugins: [],
}
