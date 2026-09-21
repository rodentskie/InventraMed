export interface ActionResult<T> {
  success: boolean
  data: T | null
  error: string | null
}
