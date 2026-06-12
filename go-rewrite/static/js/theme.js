// Shared theme controller for BMW CarData dashboard pages.
// Owns the dark/light toggle button, persistence, and a Plotly layout helper
// derived from the active CSS theme tokens. Pages listen for 'bmwthemechange'
// to re-render their Plotly charts with the new palette.
(function () {
    const root = document.documentElement;

    function current() {
        return root.getAttribute('data-theme') === 'dark' ? 'dark' : 'light';
    }

    function cssVar(name, fallback) {
        const v = getComputedStyle(root).getPropertyValue(name).trim();
        return v || fallback;
    }

    // Returns a Plotly layout fragment that matches the current theme.
    function plotlyLayout() {
        const surface = cssVar('--card-bg', '#ffffff');
        const text = cssVar('--text', '#2c3e50');
        const muted = cssVar('--muted', '#6b7785');
        const grid = cssVar('--track', '#eef1f4');
        const border = cssVar('--border', '#e6e9ee');
        return {
            paper_bgcolor: surface,
            plot_bgcolor: surface,
            font: { color: muted },
            title: { font: { color: text } },
            legend: { bgcolor: surface, bordercolor: border, font: { color: muted } },
            xaxis: {
                gridcolor: grid,
                zerolinecolor: border,
                linecolor: border,
                tickfont: { color: muted },
                title: { font: { color: muted } }
            },
            yaxis: {
                gridcolor: grid,
                zerolinecolor: border,
                linecolor: border,
                tickfont: { color: muted },
                title: { font: { color: muted } }
            }
        };
    }

    window.BMWTheme = {
        current: current,
        cssVar: cssVar,
        plotlyLayout: plotlyLayout
    };

    function setTheme(theme) {
        root.setAttribute('data-theme', theme);
        try { localStorage.setItem('bmw-theme', theme); } catch (e) { /* ignore */ }
        window.dispatchEvent(new CustomEvent('bmwthemechange', { detail: { theme: theme } }));
    }

    function wire() {
        const btn = document.getElementById('theme-toggle');
        if (btn) {
            btn.addEventListener('click', function () {
                setTheme(current() === 'dark' ? 'light' : 'dark');
            });
        }
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', wire);
    } else {
        wire();
    }
})();
