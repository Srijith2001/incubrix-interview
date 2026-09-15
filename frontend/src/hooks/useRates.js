import { useCallback, useEffect, useState } from 'react'

const DEFAULT_PAIRS = [
    { base: 'INR', target: 'EUR' },
    { base: 'INR', target: 'SGD' },
    { base: 'INR', target: 'USD' },
]
const FALLBACK_RATES = { EUR: 0.00905, SGD: 0.01329, USD: 0.01046 }

function readPairs() {
    try {
        const savedPairs = JSON.parse(localStorage.getItem('rateboard-pairs'))
        if (Array.isArray(savedPairs)) return savedPairs

        const base = localStorage.getItem('rateboard-base') || 'INR'
        const targets = JSON.parse(localStorage.getItem('rateboard-targets')) || ['EUR', 'SGD', 'USD']
        return targets.map((target) => ({ base, target }))
    } catch {
        return DEFAULT_PAIRS
    }
}

export function useRates() {
    const [pairs, setPairs] = useState(readPairs)
    const [rates, setRates] = useState(FALLBACK_RATES)
    const [isLoading, setIsLoading] = useState(false)
    const [source, setSource] = useState('sample')
    const [error, setError] = useState('')
    const [lastUpdated, setLastUpdated] = useState(new Date())

    const fetchRates = useCallback(async () => {
        if (!pairs.length) return

        setIsLoading(true)
        setError('')
        try {
            const apiBase = import.meta.env.VITE_API_BASE_URL || ''
            const groupedTargets = pairs.reduce((groups, pair) => {
                groups[pair.base] = [...(groups[pair.base] || []), pair.target]
                return groups
            }, {})
            const responses = await Promise.all(Object.entries(groupedTargets).map(async ([base, targets]) => {
                const endpoint = `${apiBase}/api/rates?base=${base}&targets=${targets.join(',')}`
                const response = await fetch(endpoint)
                if (!response.ok) throw new Error(`Request failed (${response.status})`)
                const data = await response.json()
                return { base, rates: data.rates || {} }
            }))
            setRates(Object.fromEntries(
                responses.flatMap(({ base, rates: baseRates }) => Object.entries(baseRates)
                    .map(([target, rate]) => [`${base}:${target}`, rate])),
            ))
            setSource('live')
            setLastUpdated(new Date())
        } catch {
            setRates((current) => Object.fromEntries(
                pairs.map(({ base, target }) => [
                    `${base}:${target}`,
                    current[`${base}:${target}`] || FALLBACK_RATES[target] || 0,
                ]),
            ))
            setSource('sample')
            setError('Local API unavailable. Showing sample rates.')
            setLastUpdated(new Date())
        } finally {
            setIsLoading(false)
        }
    }, [pairs])

    useEffect(() => {
        localStorage.setItem('rateboard-pairs', JSON.stringify(pairs))
        fetchRates()
    }, [pairs, fetchRates])

    const addPair = (base, target) => {
        if (!/^[A-Z]{3}$/.test(base) || !/^[A-Z]{3}$/.test(target)) return false
        if (base === target || pairs.some((pair) => pair.base === base && pair.target === target)) return false
        setPairs((current) => [...current, { base, target }])
        return true
    }

    const removePair = (pairToRemove) => {
        setPairs((current) => current.filter(
            (pair) => pair.base !== pairToRemove.base || pair.target !== pairToRemove.target,
        ))
    }

    return {
        pairs,
        rates,
        isLoading,
        source,
        error,
        lastUpdated,
        addPair,
        removePair,
        fetchRates,
    }
}
