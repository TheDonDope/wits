/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ['./**/*.html', './**/*.templ', './**/*.go'],
  theme: {
    extend: {
      container: {
        center: true,
        padding: '4px'
      }
    }
  },
  plugins: []
};
