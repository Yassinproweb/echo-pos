/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    "./views/**/*.html",
    "./views/partials/**/*.html"
  ],
  theme: {
    extend: {
      colors: {
        pos_col: 'hsl(195, 97%, 49%)',
        pos_whi: 'hsl(205, 1%, 100%)',
        pos_lgh: 'hsl(195, 1%, 57%)',
        pos_bla: 'hsl(205, 1%, 07%)',

        pos_plc: 'hsl(217, 99%, 56%)',
        pos_cld: 'hsl(357, 95%, 50%)',
        pos_trs: 'hsl(277, 81%, 57%)',
        pos_dlv: 'hsl(141, 93%, 37%)',
        pos_pdg: 'hsl(195, 1%, 67%)',
        pos_rdy: 'hsl(41, 93%, 45%)',
      },
    },
  },
  plugins: [],
}
