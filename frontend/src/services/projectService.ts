import type { ProjectDetail, ProjectInfo } from '@/types'

const api = () => window.go.main.ProjectService

export const projectService = {
  async list(): Promise<ProjectInfo[]> {
    return (await api().ListProjects()) || []
  },
  add(path: string): Promise<ProjectInfo> {
    return api().AddProject(path)
  },
  remove(path: string): Promise<void> {
    return api().RemoveProject(path)
  },
  pickDirectory(): Promise<string> {
    return api().PickProjectDirectory()
  },
  detail(path: string): Promise<ProjectDetail> {
    return api().GetProjectDetail(path)
  },
  addMcp(path: string, names: string[]): Promise<void> {
    return api().AddLibraryMcpToProject(path, names)
  },
  removeMcp(path: string, name: string): Promise<void> {
    return api().RemoveProjectMcp(path, name)
  },
  applyEnv(path: string, envName: string): Promise<string> {
    return api().ApplyEnvToProject(path, envName)
  },
  clearEnv(path: string): Promise<void> {
    return api().ClearProjectEnv(path)
  },
  saveClaudeMD(path: string, content: string): Promise<void> {
    return api().SaveProjectClaudeMD(path, content)
  },
}
