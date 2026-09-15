import './App.css'
import { BrandHeader } from './components/BrandHeader'
import { DashboardFooter } from './components/DashboardFooter'
import { RateGrid } from './components/RateGrid'
import { WatchlistForm } from './components/WatchlistForm'
import { useRates } from './hooks/useRates'

function App() {
  const {
    pairs, rates, isLoading, source, error, lastUpdated,
    addPair, removePair, fetchRates,
  } = useRates()

  return (
    <main className="app-shell">
      <BrandHeader source={source} />

      <section className="intro-row">
        <div>
          <p className="eyebrow">Your watchlist</p>
          <h2>Track the rates that matter.</h2>
          <p className="intro-copy">A quiet view of your most useful currency pairs.</p>
        </div>
      </section>

      {error && <div className="notice">{error}</div>}

      <RateGrid pairs={pairs} rates={rates} onRemove={removePair} />
      <WatchlistForm onAdd={addPair} />
      <DashboardFooter lastUpdated={lastUpdated} isLoading={isLoading} onRefresh={fetchRates} />
    </main>
  )
}

export default App
