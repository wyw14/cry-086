import { createApp, type App as VueApplication, type Plugin } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import './styles.css'
import CraneGuardShell from './App.vue'
import router from './router'

function install(application: VueApplication, extensions: Plugin[]): VueApplication {
  return extensions.reduce((current, extension) => current.use(extension), application)
}

const consoleApplication = createApp(CraneGuardShell)
install(consoleApplication, [createPinia(), router, ElementPlus]).mount('#app')
