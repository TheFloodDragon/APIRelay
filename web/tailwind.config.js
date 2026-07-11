/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{vue,js}'],
  theme: {
    colors: {
      transparent: 'transparent',
      current: 'currentColor',
      white: '#FFFFFF',
      canvas: '#F3F6FA',
      paper: '#F3F6FA',
      surface: '#FFFFFF',
      panel: '#EEF2F7',
      sidebar: '#16243A',
      ink: '#18243A',
      soft: '#627087',
      faint: '#94A0B2',
      line: '#DDE4ED',
      ghost: '#EEF2F7',
      blue: {
        DEFAULT: '#3564D4',
        deep: '#284EAE',
        wash: '#EDF2FF',
        grid: '#88A5EA',
      },
      run: { DEFAULT: '#23877F', wash: '#EAF8F6' },
      test: { DEFAULT: '#B7791F', wash: '#FFF7E6' },
      trip: { DEFAULT: '#D05A52', wash: '#FFF0EE' },
    },
    fontFamily: {
      sans: ['Segoe UI', 'PingFang SC', 'Microsoft YaHei', 'system-ui', 'sans-serif'],
      cond: ['Saira SemiCondensed', 'Segoe UI', 'PingFang SC', 'Microsoft YaHei', 'sans-serif'],
      mono: ['Spline Sans Mono', 'ui-monospace', 'Consolas', 'monospace'],
    },
    extend: {
      boxShadow: {
        sheet: '0 1px 2px rgba(22, 36, 58, 0.04), 0 8px 24px rgba(22, 36, 58, 0.035)',
        lift: '0 24px 70px rgba(17, 31, 53, 0.22)',
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
