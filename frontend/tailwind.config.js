/** @type {import('tailwindcss').Config} */
export default {
  content: [
    './index.html',
    './presenter.html',
    './src/**/*.{svelte,js,ts,jsx,tsx}',
  ],
  theme: {
    extend: {
      // Presentation-specific font sizes (based on 1920px base width)
      fontSize: {
        'slide-title': ['8rem', { lineHeight: '1.1', letterSpacing: '-0.02em' }],
        'slide-h1': ['6rem', { lineHeight: '1.15', letterSpacing: '-0.02em' }],
        'slide-h2': ['4rem', { lineHeight: '1.2', letterSpacing: '-0.01em' }],
        'slide-h3': ['2.5rem', { lineHeight: '1.25' }],
        'slide-body': ['2rem', { lineHeight: '1.5' }],
        'slide-code': ['1.5rem', { lineHeight: '1.6' }],
        'slide-stat': ['12rem', { lineHeight: '1', letterSpacing: '-0.03em' }],
        'slide-caption': ['1.25rem', { lineHeight: '1.5' }],
        'slide-small': ['1rem', { lineHeight: '1.5' }],
      },

      // Font family stacks for presentations
      fontFamily: {
        'sans': ['Inter', 'system-ui', '-apple-system', 'BlinkMacSystemFont', 'Segoe UI', 'Roboto', 'Helvetica Neue', 'Arial', 'sans-serif'],
        'mono': ['JetBrains Mono', 'SF Mono', 'Monaco', 'Cascadia Code', 'Consolas', 'Liberation Mono', 'Menlo', 'monospace'],
        'display': ['Space Grotesk', 'Inter', 'system-ui', 'sans-serif'],
        'serif': ['Playfair Display', 'Georgia', 'Cambria', 'Times New Roman', 'serif'],
        'poster': ['Anton', 'Impact', 'Haettenschweiler', 'Arial Narrow Bold', 'sans-serif'],
      },

      // Slide-appropriate spacing values
      spacing: {
        'slide': '80px',
        'slide-sm': '40px',
        'slide-lg': '120px',
        'slide-xl': '160px',
        'slide-2xl': '200px',
        'content-gap': '2rem',
        'content-gap-lg': '3rem',
        'content-gap-xl': '4rem',
      },

      // Aspect ratio utilities for presentations
      aspectRatio: {
        '16/9': '16 / 9',
        '4/3': '4 / 3',
        '16/10': '16 / 10',
        'slide': '16 / 9',
      },

      // Animation and transition timing tokens
      transitionDuration: {
        'slide': '400ms',
        'slide-fast': '200ms',
        'slide-slow': '600ms',
        'fragment': '300ms',
      },
      transitionTimingFunction: {
        'slide': 'cubic-bezier(0.4, 0, 0.2, 1)',
        'slide-in': 'cubic-bezier(0, 0, 0.2, 1)',
        'slide-out': 'cubic-bezier(0.4, 0, 1, 1)',
        'spring': 'cubic-bezier(0.34, 1.56, 0.64, 1)',
      },
      animation: {
        'fade-in': 'fadeIn var(--transition-duration, 400ms) var(--transition-timing, ease-out)',
        'fade-out': 'fadeOut var(--transition-duration, 400ms) var(--transition-timing, ease-out)',
        'slide-up': 'slideUp var(--transition-duration, 400ms) var(--transition-timing, ease-out)',
        'slide-down': 'slideDown var(--transition-duration, 400ms) var(--transition-timing, ease-out)',
        'scale-in': 'scaleIn var(--transition-duration, 400ms) var(--transition-timing, ease-out)',
        'pulse-glow': 'pulseGlow 2s ease-in-out infinite',
      },
      keyframes: {
        fadeIn: {
          '0%': { opacity: '0' },
          '100%': { opacity: '1' },
        },
        fadeOut: {
          '0%': { opacity: '1' },
          '100%': { opacity: '0' },
        },
        slideUp: {
          '0%': { opacity: '0', transform: 'translateY(20px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' },
        },
        slideDown: {
          '0%': { opacity: '0', transform: 'translateY(-20px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' },
        },
        scaleIn: {
          '0%': { opacity: '0', transform: 'scale(0.95)' },
          '100%': { opacity: '1', transform: 'scale(1)' },
        },
        pulseGlow: {
          '0%, 100%': { boxShadow: '0 0 10px currentColor' },
          '50%': { boxShadow: '0 0 20px currentColor, 0 0 30px currentColor' },
        },
      },

      // CSS custom property bridge for theme switching
      // These colors reference CSS variables that themes will define
      colors: {
        theme: {
          bg: 'var(--color-bg)',
          text: 'var(--color-text)',
          muted: 'var(--color-muted)',
          accent: 'var(--color-accent)',
          'code-bg': 'var(--color-code-bg)',
          border: 'var(--color-border, var(--color-muted))',
          surface: 'var(--color-surface, var(--color-bg))',
          'surface-elevated': 'var(--color-surface-elevated, var(--color-surface))',
        },
      },

      // Border radius tokens
      borderRadius: {
        'slide': '8px',
        'slide-lg': '12px',
        'slide-xl': '16px',
      },

      // Box shadow tokens for depth
      boxShadow: {
        'slide': '0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -1px rgba(0, 0, 0, 0.06)',
        'slide-lg': '0 10px 15px -3px rgba(0, 0, 0, 0.1), 0 4px 6px -2px rgba(0, 0, 0, 0.05)',
        'slide-xl': '0 20px 25px -5px rgba(0, 0, 0, 0.1), 0 10px 10px -5px rgba(0, 0, 0, 0.04)',
        'glow': '0 0 10px var(--color-accent, currentColor)',
        'glow-lg': '0 0 20px var(--color-accent, currentColor), 0 0 30px var(--color-accent, currentColor)',
      },

      // Backdrop blur for glassmorphism
      backdropBlur: {
        'glass': '8px',
        'glass-lg': '16px',
        'glass-xl': '24px',
      },
    },
  },
  plugins: [],
}
