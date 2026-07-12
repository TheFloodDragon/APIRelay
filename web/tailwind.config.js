/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{vue,js}'],
  theme: {
    colors: {
      transparent: 'transparent',
      current: 'currentColor',
      white: '#FFFFFF',
      canvas: '#F4F7FA',
      paper: '#F4F7FA',
      surface: '#FFFFFF',
      panel: '#EDF2F6',
      sidebar: '#142033',
      ink: '#142033',
      soft: '#5E6E83',
      faint: '#8C99AA',
      line: '#DCE4EC',
      ghost: '#EDF2F6',
      blue: {
        DEFAULT: '#2F63D8',
        deep: '#244EAE',
        wash: '#EBF1FF',
        grid: '#86A5EB',
      },
      run: { DEFAULT: '#27858A', wash: '#E8F6F5' },
      test: { DEFAULT: '#C8842E', wash: '#FFF5E7' },
      trip: { DEFAULT: '#D45F59', wash: '#FFF0EF' },
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
