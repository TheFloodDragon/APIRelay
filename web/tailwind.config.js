/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{vue,js}'],
  theme: {
    colors: {
      transparent: 'transparent',
      current: 'currentColor',
      white: '#FFFFFF',
      canvas: '#F5F7FA',
      paper: '#F5F7FA',
      surface: '#FFFFFF',
      panel: '#F2F4F7',
      sidebar: '#18212F',
      ink: '#172033',
      soft: '#667085',
      faint: '#98A2B3',
      line: '#E4E7EC',
      ghost: '#F2F4F7',
      blue: {
        DEFAULT: '#2563EB',
        deep: '#1D4ED8',
        wash: '#EFF6FF',
        grid: '#DBEAFE',
      },
      run: { DEFAULT: '#15803D', wash: '#F0FDF4' },
      test: { DEFAULT: '#B45309', wash: '#FFFBEB' },
      trip: { DEFAULT: '#B42318', wash: '#FEF3F2' },
    },
    fontFamily: {
      sans: ['Segoe UI', 'PingFang SC', 'Microsoft YaHei', 'system-ui', 'sans-serif'],
      cond: ['Saira SemiCondensed', 'Segoe UI', 'PingFang SC', 'Microsoft YaHei', 'sans-serif'],
      mono: ['Spline Sans Mono', 'ui-monospace', 'Consolas', 'monospace'],
    },
    extend: {
      boxShadow: {
        sheet: '0 1px 2px rgba(16, 24, 40, 0.04)',
        lift: '0 18px 45px rgba(16, 24, 40, 0.16)',
      },
      fontSize: {
        '2xs': ['0.6875rem', { lineHeight: '1rem' }],
      },
      borderRadius: {
        DEFAULT: '0.5rem',
      },
    },
  },
  plugins: [],
}
