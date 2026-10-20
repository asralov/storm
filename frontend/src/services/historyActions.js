import { deleteHistory } from './testService'

export async function deleteSavedReport(item, confirmDelete = message => window.confirm(message), remove = deleteHistory) {
  const target = item.url || item.config?.url || item.id
  if (!confirmDelete(`Delete the saved report for ${target}? This cannot be undone.`)) return false
  await remove(item.id)
  if (typeof window !== 'undefined') window.dispatchEvent(new CustomEvent('storm-history-deleted', { detail: { id: item.id } }))
  return true
}
