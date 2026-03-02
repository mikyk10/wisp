import { apiClient } from './client'
import { API_PATHS } from '@/config'

interface CatalogsResponse {
  catalogs: string[]
}

export const catalogsApi = {
  async fetchAll(): Promise<string[]> {
    const { data } = await apiClient.get<CatalogsResponse>(API_PATHS.catalogs())
    return data.catalogs ?? []
  },
}
