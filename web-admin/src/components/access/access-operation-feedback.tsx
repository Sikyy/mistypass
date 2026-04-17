type AccessOperationFeedbackProps = {
  error: string
  summary: string
}

export function AccessOperationFeedback({ error, summary }: AccessOperationFeedbackProps) {
  return (
    <>
      {error ? (
        <div className="rounded-lg border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {error}
        </div>
      ) : null}
      {summary ? (
        <div className="rounded-lg border border-emerald-500/40 bg-emerald-500/10 px-3 py-2 text-sm text-emerald-700">
          {summary}
        </div>
      ) : null}
    </>
  )
}
