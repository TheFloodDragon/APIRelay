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

        text: v('--c-text'),
        'text-dim': v('--c-text-dim'),
        'text-muted': v('--c-text-muted'),

        primary: v('--c-primary'),
        accent: v('--c-accent'),
        success: v('--c-success'),
        warning: v('--c-warning'),
        danger: v('--c-danger'),
      },
      borderRadius: {
        DEFAULT: '8px',
        lg: '12px',
        xl: '16px',
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
