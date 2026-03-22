/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        // Bank-themed colors
        hdfc: { DEFAULT: '#004B87', light: '#0066B3' },
        icici: { DEFAULT: '#F58220', light: '#FF9933' },
        sbi: { DEFAULT: '#22409A', light: '#3355BB' },
        amex: { DEFAULT: '#006FCF', light: '#1A8FE3' },
        axis: { DEFAULT: '#97144D', light: '#B91C5C' },
      },
    },
  },
  plugins: [],
}
