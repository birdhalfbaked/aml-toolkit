/** Normalize thrown values for display in alerts and snackbars. */
export function errorMessage(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}
