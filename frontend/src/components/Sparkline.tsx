/**
 * A minimal SVG sparkline.
 *
 * The sidebar renders one of these per instance at one update per second;
 * a charting library would be far heavier than the few lines it takes here.
 */
export function Sparkline({
  values,
  width = 72,
  height = 20,
  className,
}: {
  values: readonly number[]
  width?: number
  height?: number
  className?: string
}) {
  if (values.length < 2) {
    return (
      <svg width={width} height={height} className={className} aria-hidden="true">
        <line
          x1={0}
          y1={height - 1}
          x2={width}
          y2={height - 1}
          stroke="currentColor"
          strokeWidth={1}
          opacity={0.25}
        />
      </svg>
    )
  }

  const maximum = Math.max(...values, 0)
  const range = maximum > 0 ? maximum : 1
  const step = width / (values.length - 1)

  const points = values.map((value, index) => {
    const x = index * step
    // Inset by one pixel so a flat maximum is not clipped by the viewBox edge.
    const y = height - 1 - (value / range) * (height - 2)
    return `${x.toFixed(1)},${y.toFixed(1)}`
  })

  const area = `0,${height} ${points.join(' ')} ${width},${height}`

  return (
    <svg width={width} height={height} className={className} aria-hidden="true">
      <polygon points={area} fill="currentColor" opacity={0.14} />
      <polyline
        points={points.join(' ')}
        fill="none"
        stroke="currentColor"
        strokeWidth={1.5}
        strokeLinejoin="round"
        strokeLinecap="round"
      />
    </svg>
  )
}
