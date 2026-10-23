import { useEffect } from 'react'
import { useQueryClient } from '@tanstack/react-query'

export function useEvents() {
  const queryClient = useQueryClient()
  useEffect(() => {
    const source = new EventSource('/api/v1/events')
    const invalidate = () => {
      queryClient.invalidateQueries({ queryKey: ['repos'] })
      queryClient.invalidateQueries({ queryKey: ['settings'] })
      queryClient.invalidateQueries({ queryKey: ['repo'] })
      queryClient.invalidateQueries({ queryKey: ['syncruns'] })
    }
    for (const type of ['repo.updated', 'sync.started', 'sync.finished', 'sync.failed']) {
      source.addEventListener(type, invalidate)
    }
    return () => source.close()
  }, [queryClient])
}
