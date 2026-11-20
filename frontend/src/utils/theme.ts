import { ref } from 'vue'

const isDark = ref(false)

export function useTheme() {
  function initTheme() {
    const saved = localStorage.getItem('rag_studio_theme')
    // Default to dark mode as explicitly requested by user
    const shouldBeDark = saved ? saved === 'dark' : true
    setDark(shouldBeDark)
  }

  function setDark(val: boolean) {
    isDark.value = val
    if (val) {
      document.documentElement.setAttribute('data-theme', 'dark')
      document.body.classList.add('dark-mode')
      localStorage.setItem('rag_studio_theme', 'dark')
    } else {
      document.documentElement.removeAttribute('data-theme')
      document.body.classList.remove('dark-mode')
      localStorage.setItem('rag_studio_theme', 'light')
    }
  }

  function toggleTheme() {
    setDark(!isDark.value)
  }

  return {
    isDark,
    initTheme,
    setDark,
    toggleTheme
  }
}
