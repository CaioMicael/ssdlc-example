import { useHealth } from './useHealth'
import { useTasks } from './useTasks'
import { TaskForm } from './TaskForm'
import { TaskList } from './TaskList'

const STATUS_TEXT = {
  loading: 'Verificando API...',
  online: 'API online',
  offline: 'API indisponível',
} as const

function App() {
  const health = useHealth()
  const { tasks, error, showArchived, create, changeStatus, archive, restore, toggleArchived } =
    useTasks()

  return (
    <>
      <h1>SSDLC Example</h1>
      <p>{STATUS_TEXT[health]}</p>
      <TaskForm onCreate={create} />
      {error && <p role="alert">{error}</p>}
      <TaskList
        tasks={tasks}
        showArchived={showArchived}
        onChangeStatus={changeStatus}
        onArchive={archive}
        onRestore={restore}
        onToggleArchived={toggleArchived}
      />
    </>
  )
}

export default App
