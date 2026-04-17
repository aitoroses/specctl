import { QueryClient, useQuery } from '@tanstack/react-query'
import { fetchJson, getStaticData, isStaticMode } from './client'
import type { OverviewData, GraphData, SpecDetail } from './types'

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: isStaticMode ? Infinity : 30_000,
      retry: isStaticMode ? false : 3,
    },
  },
})

export function useOverview() {
  return useQuery<OverviewData>({
    queryKey: ['overview'],
    queryFn: () => fetchJson<OverviewData>('/api/overview'),
    ...(isStaticMode && { initialData: getStaticData<OverviewData>('overview') }),
  })
}

export function useSpec(charter: string, slug: string) {
  return useQuery<SpecDetail>({
    queryKey: ['spec', charter, slug],
    queryFn: () => fetchJson<SpecDetail>(`/api/specs/${charter}/${slug}`),
    enabled: Boolean(charter && slug),
  })
}

export function useGraph() {
  return useQuery<GraphData>({
    queryKey: ['graph'],
    queryFn: () => fetchJson<GraphData>('/api/graph'),
    ...(isStaticMode && { initialData: getStaticData<GraphData>('graph') }),
  })
}

