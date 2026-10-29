import React from 'react'
import {createRoot} from 'react-dom/client'
import './i18n'
import './style.css'
import App from './App'
import ThemeProvider from './ThemeProvider'

const container = document.getElementById('root')

const root = createRoot(container!)

root.render(
    <React.StrictMode>
        <ThemeProvider><App/></ThemeProvider>
    </React.StrictMode>
)
