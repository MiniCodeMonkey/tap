import { mount } from 'svelte'
import './app.css'
import Presenter from './Presenter.svelte'

const app = mount(Presenter, {
  target: document.getElementById('app')!,
})

export default app
