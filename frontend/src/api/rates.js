const apiBase = import.meta.env.VITE_API_BASE_URL || ''

export async function fetchRatesForPairs(pairs) {
    const groupedTargets = pairs.reduce((groups, pair) => {
        groups[pair.base] = [...(groups[pair.base] || []), pair.target]
        return groups
    }, {})

    const responses = await Promise.all(
        Object.entries(groupedTargets).map(async ([base, targets]) => {
            const endpoint = `${apiBase}/api/rates?base=${base}&targets=${targets.join(',')}`
            const response = await fetch(endpoint)
            if (!response.ok) throw new Error(`Request failed (${response.status})`)
            const data = await response.json()
            return { base, rates: data.rates || {} }
        }),
    )

    return Object.fromEntries(
        responses.flatMap(({ base, rates }) => Object.entries(rates)
            .map(([target, rate]) => [`${base}:${target}`, rate])),
    )
}
