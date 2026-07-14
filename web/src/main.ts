import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import './styles/main.css'

const savedTheme = window.localStorage.getItem('marketpulse-theme') === 'light' ? 'light' : 'dark'
document.documentElement.dataset.theme = savedTheme

createApp(App).use(createPinia()).use(router).mount('#app')
