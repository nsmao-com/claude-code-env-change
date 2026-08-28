import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';
import tailwindcss from '@tailwindcss/vite';
import { resolve } from 'path';
export default defineConfig({
    plugins: [vue(), tailwindcss()],
    resolve: {
        alias: {
            '@': resolve(__dirname, 'src'),
        },
    },
    server: {
        port: 5173,
        strictPort: true,
        host: 'localhost',
        origin: 'http://localhost:5173',
        cors: true,
    },
    build: {
        outDir: 'dist',
        emptyOutDir: true,
        rollupOptions: {
            output: {
                manualChunks: function (id) {
                    if (!id.includes('node_modules'))
                        return;
                    if (id.includes('chart.js') || id.includes('vue-chartjs'))
                        return 'chart';
                    if (id.includes('codemirror') || id.includes('@lezer'))
                        return 'codemirror';
                    if (id.includes('@headlessui'))
                        return 'headless';
                    if (id.includes('sortablejs'))
                        return 'sortable';
                    if (id.includes('/vue/') || id.includes('\\vue\\') || id.includes('pinia'))
                        return 'vue';
                    return 'vendor';
                }
            }
        }
    },
});
