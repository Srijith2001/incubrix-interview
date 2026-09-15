export function DashboardFooter({ lastUpdated, isLoading, onRefresh }) {
    return (
        <footer className="footer-row">
            <span>Updated {lastUpdated.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}</span>
            <button type="button" className="refresh-button" onClick={onRefresh} disabled={isLoading}>
                {isLoading ? 'Refreshing...' : 'Refresh rates'} <span aria-hidden="true">&#8635;</span>
            </button>
        </footer>
    )
}
