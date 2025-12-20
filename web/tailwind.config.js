/** @type {import('tailwindcss').Config} */
export default {
    content: [
        "./index.html",
        "./src/**/*.{js,ts,jsx,tsx}",
    ],
    theme: {
        extend: {
            fontFamily: {
                inter: ['Inter', 'sans-serif'],
            },
            colors: {
                background: "#0f0f13",
                surface: "#18181b",
                surfaceHover: "#27272a",
                primary: "#6366f1", // Indigo 500
                primaryHover: "#4f46e5",
                accent: "#8b5cf6", // Violet 500
                text: "#faFAFA",
                textMuted: "#a1a1aa",
                border: "#27272a",
            }
        },
    },
    plugins: [],
}
