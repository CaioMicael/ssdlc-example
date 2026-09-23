import { useState } from 'react'
import type { Task, TaskStatus } from './api/tasks'
import { TimeEntries } from './TimeEntries'
import { formatHm } from './timeFormat'

const STATUS_LABEL: Record<TaskStatus, string> = {
  todo: 'A fazer',
  in_progress: 'Em andamento',
  done: 'Concluída',
}

interface TaskListProps {
  tasks: Task[]
  showArchived: boolean
  onChangeStatus: (id: string, status: TaskStatus) => void
  onArchive: (id: string) => void
  onRestore: (id: string) => void
  onToggleArchived: () => void
  onStartTimer: (id: string) => void
}

export function TaskList({
  tasks,
  showArchived,
  onChangeStatus,
  onArchive,
  onRestore,
  onToggleArchived,
  onStartTimer,
}: Readonly<TaskListProps>) {
  const [expandedId, setExpandedId] = useState<string | null>(null)

  return (
    <div>
      <label>
        <input type="checkbox" checked={showArchived} onChange={onToggleArchived} />{' '}
        <span>Mostrar arquivadas</span>
      </label>
      {tasks.length === 0 ? (
        <p>Nenhuma tarefa ainda</p>
      ) : (
        <ul>
          {tasks.map((task) => (
            <li key={task.id}>
              <span>{task.title}</span>
              <select
                aria-label={`Status de ${task.title}`}
                value={task.status}
                onChange={(e) => onChangeStatus(task.id, e.target.value as TaskStatus)}
              >
                <option value="todo">{STATUS_LABEL.todo}</option>
                <option value="in_progress">{STATUS_LABEL.in_progress}</option>
                <option value="done">{STATUS_LABEL.done}</option>
              </select>
              <span>Total: {formatHm(task.total_seconds)}</span>
              {task.status !== 'done' && !task.archived && (
                <button type="button" onClick={() => onStartTimer(task.id)}>
                  Iniciar
                </button>
              )}
              {showArchived ? (
                <button type="button" onClick={() => onRestore(task.id)}>
                  Restaurar
                </button>
              ) : (
                <button type="button" onClick={() => onArchive(task.id)}>
                  Arquivar
                </button>
              )}
              <button
                type="button"
                onClick={() => setExpandedId((prev) => (prev === task.id ? null : task.id))}
              >
                {expandedId === task.id ? 'Ocultar apontamentos' : 'Ver apontamentos'}
              </button>
              {expandedId === task.id && <TimeEntries taskId={task.id} />}
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
