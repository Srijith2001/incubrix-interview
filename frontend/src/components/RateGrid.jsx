import { RateCard } from './RateCard'

export function RateGrid({ pairs, rates, onRemove }) {
    return (
        <section className="rate-grid" aria-live="polite">
            {pairs.map(({ base, target }, index) => (
                <RateCard
                    key={`${base}:${target}`}
                    base={base}
                    target={target}
                    rate={rates[`${base}:${target}`]}
                    delay={index * 70}
                    onRemove={onRemove}
                />
            ))}
            {!pairs.length && <div className="empty-state">Add a currency pair to start tracking.</div>}
        </section>
    )
}
