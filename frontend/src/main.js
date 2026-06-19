import { createApp } from 'vue'
import { createPinia } from 'pinia'
import {
  Chart,
  LineController,
  LineElement,
  PointElement,
  LinearScale,
  CategoryScale,
  Filler,
  Tooltip,
  Legend,
} from 'chart.js'

import App from './App.vue'
import router from './router'
import './style.css'

// Register only the Chart.js pieces we use to keep the bundle small.
Chart.register(
  LineController,
  LineElement,
  PointElement,
  LinearScale,
  CategoryScale,
  Filler,
  Tooltip,
  Legend,
)

createApp(App).use(createPinia()).use(router).mount('#app')
