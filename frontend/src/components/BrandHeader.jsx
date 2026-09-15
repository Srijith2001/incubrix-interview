export function BrandHeader({ source }) {
    return (
        <header className="topbar">
            <div className="brand-lockup">
                <span className="brand-mark">R</span>
                <div>
                    <p className="eyebrow">Currency Watcher</p>
                    <h1>Rateboard</h1>
                </div>
            </div>
            <div className={`connection-pill ${source}`}>
                <span className="status-dot" />
                {source === 'live' ? 'Local API connected' : 'Using sample data'}
            </div>
        </header>
    )
}
