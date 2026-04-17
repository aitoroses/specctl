export const isStaticMode = typeof window !== 'undefined' && window.__SPECCTL_DATA__ !== undefined

export async function fetchJson<T>(url: string): Promise<T> {
  const response = await fetch(url)
  if (!response.ok) {
    throw new Error(`HTTP ${response.status}: ${response.statusText}`)
  }
  return response.json() as Promise<T>
}

export function getStaticData<T>(key: string): T | undefined {
  if (!isStaticMode || !window.__SPECCTL_DATA__) return undefined
  return window.__SPECCTL_DATA__[key] as T | undefined
}
