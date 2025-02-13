/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ['./view/**/*.templ', './**/*.templ'],
  theme: {
    extend: {}
  },
  daisyui: {
    themes: ['lemonade', 'forest']
  }
};
