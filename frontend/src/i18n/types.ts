// 语言包结构约束：英文包必须与中文包的键一一对应（多键、漏键都会在类型检查时报错）
export type DeepString<T> = {
  [K in keyof T]: T[K] extends string ? string : DeepString<T[K]>
}
