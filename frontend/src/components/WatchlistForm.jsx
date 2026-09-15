import { useState } from 'react'

export function WatchlistForm({ onAdd }) {
    const [newBase, setNewBase] = useState('INR')
    const [newTarget, setNewTarget] = useState('')

    const handleSubmit = (event) => {
        event.preventDefault()
        const base = newBase.trim().toUpperCase()
        const target = newTarget.trim().toUpperCase()
        if (onAdd(base, target)) setNewTarget('')
    }

    return (
        <section className="manage-panel">
            <div>
                <p className="eyebrow">Manage watchlist</p>
                <h3>Add another currency</h3>
            </div>
            <form className="add-form" onSubmit={handleSubmit}>
                <label className="sr-only" htmlFor="new-base">Base currency</label>
                <input
                    id="new-base"
                    value={newBase}
                    onChange={(event) => setNewBase(event.target.value.toUpperCase())}
                    placeholder="INR"
                    maxLength="3"
                />
                <span className="pair-arrow" aria-hidden="true">&#8594;</span>
                <label className="sr-only" htmlFor="new-currency">Target currency</label>
                <input
                    id="new-currency"
                    value={newTarget}
                    onChange={(event) => setNewTarget(event.target.value.toUpperCase())}
                    placeholder="GBP"
                    maxLength="3"
                />
                <button type="submit">Add pair <span>+</span></button>
            </form>
        </section>
    )
}
