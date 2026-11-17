import './styles/app.css'
import { mount } from 'svelte'
import App from './App.svelte'
import { boot } from './lib/state.svelte'

boot()
const app = mount(App, { target: document.getElementById('app')! })
export default app
