import { useHealth } from './useHealth'

const STATUS_TEXT = {
  loading: 'Verificando API...',
  online: 'API online',
  offline: 'API indisponível',
} as const

function App() {
  const health = useHealth()

  return (
    <>
      <h1>SSDLC Example</h1>
      <p>{STATUS_TEXT[health]}</p>
    </>
  )
}

export default App
