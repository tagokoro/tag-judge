import type { Config } from "tailwindcss";

const config: Config = {
  content: ["./app/**/*.{ts,tsx}", "./components/**/*.{ts,tsx}", "./lib/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        ink: "#17201d",
        line: "#d7ded8",
        panel: "#f7f8f6",
        accent: "#0f766e",
        warn: "#a16207",
        danger: "#b91c1c"
      },
      boxShadow: {
        soft: "0 1px 2px rgba(23, 32, 29, 0.08)"
      }
    }
  },
  plugins: []
};

export default config;
