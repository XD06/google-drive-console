/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{js,ts,jsx,tsx}"],
  theme: {
    extend: {
      colors: {
        surface: "#f4f5f7",
        panel: "#ffffff",
        ink: "#1a1d24",
        muted: "#6b7280",
        line: "#e4e6eb",
        brand: {
          DEFAULT: "#0d9488",
          soft: "#ccfbf1",
          dark: "#0f766e",
        },
      },
      fontFamily: {
        sans: [
          "ui-sans-serif",
          "system-ui",
          "-apple-system",
          "Segoe UI",
          "sans-serif",
        ],
      },
      boxShadow: {
        panel: "0 1px 2px rgba(16,24,40,.04), 0 8px 24px rgba(16,24,40,.06)",
      },
    },
  },
  plugins: [],
};
