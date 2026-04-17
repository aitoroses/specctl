interface GherkinBlockProps {
  gherkin: string
}

const KEYWORDS = ['GIVEN', 'WHEN', 'THEN', 'AND', 'BUT'] as const

interface ParsedStep {
  keyword: string
  rest: string
}

function parseGherkin(gherkin: string): ParsedStep[] {
  const lines = gherkin.split('\n')
  const steps: ParsedStep[] = []

  for (const raw of lines) {
    const trimmed = raw.trim()
    if (!trimmed) continue

    const found = KEYWORDS.find(kw => trimmed.startsWith(kw + ' ') || trimmed.startsWith(kw + ':'))
    if (found) {
      steps.push({ keyword: found, rest: trimmed.slice(found.length).trim() })
    } else {
      // Non-keyword lines (Feature:, Scenario:, plain text)
      steps.push({ keyword: '', rest: trimmed })
    }
  }

  return steps
}

export function GherkinBlock({ gherkin }: GherkinBlockProps) {
  const steps = parseGherkin(gherkin)

  return (
    <div className="bg-surface-container-lowest p-6 font-mono text-xs">
      <div className="flex gap-4">
        <div className="w-1 bg-primary/30 rounded-full" />
        <div className="space-y-1.5 py-1">
          {steps.map((step, i) => (
            <p key={i}>
              {step.keyword ? (
                <>
                  <span className="text-primary-container">{step.keyword}</span>{' '}
                  <span className="text-on-surface-variant">{step.rest}</span>
                </>
              ) : (
                <span className="text-on-surface-variant">{step.rest}</span>
              )}
            </p>
          ))}
        </div>
      </div>
    </div>
  )
}
