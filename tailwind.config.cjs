module.exports = {
      content: ['./internal/web/ui/**/*.templ', './assets/js/**/*.js'],
      darkMode: 'class',
      theme: {
        extend: {
          colors: {
            brand: {
              50: '#ecfeff',
              100: '#cffafe',
              200: '#a5f3fc',
              300: '#67e8f9',
              400: '#22d3ee',
              500: '#00ADD8',
              600: '#0891b2',
              700: '#0e7490',
            },
            dark: {
              bg: '#080c14',
              card: '#111724',
              surface: '#172032',
              border: '#1f2a3f',
              input: '#0d1320',
              hover: '#1a2438'
            }
          },
          fontFamily: {
            sans: ['Inter', '-apple-system', 'BlinkMacSystemFont', 'sans-serif'],
            mono: ['JetBrains Mono', 'monospace'],
          }
        }
      }
    };
