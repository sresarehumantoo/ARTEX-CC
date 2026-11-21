import { useEffect } from 'react'
import { useNavigate } from 'react-router-dom'

export function useKeyboardShortcuts() {
  const navigate = useNavigate()

  useEffect(() => {
    function handleKey(e: KeyboardEvent) {
      const tag = (e.target as HTMLElement).tagName
      if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return

      if (e.key === '1' && !e.ctrlKey && !e.metaKey) navigate('/')
      if (e.key === '2' && !e.ctrlKey && !e.metaKey) navigate('/graph')
      if (e.key === '3' && !e.ctrlKey && !e.metaKey) navigate('/findings')
      if (e.key === '4' && !e.ctrlKey && !e.metaKey) navigate('/traffic')
      if (e.key === '5' && !e.ctrlKey && !e.metaKey) navigate('/assets')

      if (e.key === '?' && !e.ctrlKey && !e.metaKey) {
        window.dispatchEvent(new CustomEvent('toggle-shortcuts-help'))
      }
    }
    document.addEventListener('keydown', handleKey)
    return () => document.removeEventListener('keydown', handleKey)
  }, [navigate])
}
