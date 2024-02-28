/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ['./view/**/*.templ', './**/*.templ'],
  theme: {
    extend: {}
  },
  plugins: [require('daisyui')],
  daisyui: {
    themes: ['lemonade', 'forest']
  }
};
