const formatRate = (rate) => rate.toFixed(5)

export function RateCard({ base, target, rate, delay, onRemove }) {
    const hasRate = Boolean(rate)

    return (
        <article className="rate-card" style={{ '--delay': `${delay}ms` }}>
            <div className="card-heading">
                <div className="currency-icon">{target.slice(0, 1)}</div>
                <button type="button" className="remove-button" onClick={() => onRemove({ base, target })} aria-label={`Remove ${base} to ${target}`}>
                    x
                </button>
            </div>
            <p className="pair-label">{base} {`->`} {target}</p>
            <p className="rate-value">{hasRate ? formatRate(rate) : '--'}</p>
            <p className="rate-caption">
                1 {base} equals {hasRate ? `${formatRate(rate)} ${target}` : 'loading'}
            </p>
        </article>
    )
}
