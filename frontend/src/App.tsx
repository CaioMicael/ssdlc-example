import { useHealth } from './useHealth'
import { useTasks } from './useTasks'
import { useTimer } from './useTimer'
import { TaskForm } from './TaskForm'
import { TaskList } from './TaskList'
import { formatHms } from './timeFormat'

const STATUS_TEXT = {
  loading: 'Verificando API...',
  online: 'API online',
  offline: 'API indisponível',
} as const

function App() {
  const health = useHealth()
  const { tasks, error, showArchived, create, changeStatus, archive, restore, toggleArchived } =
    useTasks()
  const { activeEntry, activeTask, elapsedSeconds, start, stop } = useTimer()

  return (
    <main className="app">
      <h1>SSDLC Example</h1>
      <p className="health">{STATUS_TEXT[health]}</p>
      {activeEntry && activeTask && (
        <output className="timerbar">
          <span className="timerbar__task">{activeTask.title}</span>
          <span className="timerbar__time">{formatHms(elapsedSeconds)}</span>
          <button type="button" onClick={() => stop()}>
            Parar
          </button>
        </output>
      )}
      <TaskForm onCreate={create} />
      {error && <p role="alert">{error}</p>}
      <TaskList
        tasks={tasks}
        showArchived={showArchived}
        onChangeStatus={changeStatus}
        onArchive={archive}
        onRestore={restore}
        onToggleArchived={toggleArchived}
        onStartTimer={(id) => start(id)}
      />
    </main>
  )
}

export default App
