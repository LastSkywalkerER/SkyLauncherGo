/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        bg: "#0f0f12",
        surface: "#16161b",
        accent: "#7c5cff",
      },
    },
  },
  plugins: [],
};
