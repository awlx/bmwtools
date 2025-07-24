// BMW Tools - Anonymous Statistics Dashboard

// Plotly template for better styling (copied from main app.js)
const plotlyTemplate = {
    layout: {
        paper_bgcolor: '#ffffff',
        plot_bgcolor: '#ffffff',
        font: {
            family: 'SF Pro Display, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif',
            color: '#444',
            size: 12
        },
        title: {
            font: {
                family: 'SF Pro Display, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif',
                size: 18,
                color: '#333'
            }
        },
        colorway: ['#3498db', '#2ecc71', '#f39c12', '#e74c3c', '#9b59b6', '#1abc9c', '#34495e', '#7f8c8d', '#d35400', '#c0392b'],
        legend: {
            bgcolor: '#ffffff',
            bordercolor: '#f0f0f0',
            borderwidth: 1,
            font: {
                family: 'SF Pro Display, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif',
                size: 12,
                color: '#555'
            }
        },
        xaxis: {
            gridcolor: '#f0f0f0',
            zerolinecolor: '#e0e0e0'
        },
        yaxis: {
            gridcolor: '#f0f0f0',
            zerolinecolor: '#e0e0e0'
        }
    }
};

console.log('stats.js loaded - ' + new Date().toISOString());

// Initialize the statistics dashboard immediately and also on DOMContentLoaded
initStatsDashboard();  // Try to initialize immediately

document.addEventListener('DOMContentLoaded', function() {
    console.log('DOMContentLoaded event triggered for stats page');
    initStatsDashboard();  // Try again after DOM is loaded
});

// Main initialization function
async function initStatsDashboard() {
    console.log('Initializing stats dashboard, fetching data from /api/anonymous-stats');
    try {
        // Fetch statistics data
        const response = await fetch('/api/anonymous-stats');
        
        if (!response.ok) {
            throw new Error('Failed to fetch statistics data');
        }
        
        const data = await response.json();
        
        // Update the UI with the data
        updateGlobalStats(data.global_stats, data.soc_stats);
        updateProviderTable(data.providers);
        createProviderFailureChart(data.providers);
        createSessionOutcomesChart(data.global_stats);
        createEnergyByProviderChart(data.providers);
        
    } catch (error) {
        console.error('Error initializing stats dashboard:', error);
        showErrorMessage('Failed to load statistics data. Please try again later.');
    }
}

// Update global statistics section
function updateGlobalStats(globalStats, socStats) {
    // Update total sessions
    document.getElementById('total-sessions-count').textContent = globalStats.total_sessions.toLocaleString();
    
    // Update success rate
    const successRate = document.getElementById('success-rate');
    successRate.textContent = globalStats.success_rate.toFixed(1) + '%';
    
    // Set color based on success rate
    if (globalStats.success_rate >= 80) {
        successRate.classList.add('success-high');
    } else if (globalStats.success_rate >= 50) {
        successRate.classList.add('success-medium');
    } else {
        successRate.classList.add('success-low');
    }
    
    // Update total energy
    document.getElementById('total-energy').textContent = globalStats.total_energy_added.toFixed(0) + ' kWh';
    
    // Update SOC statistics
    document.getElementById('avg-start-soc').textContent = socStats.average_start_soc.toFixed(1) + '%';
    document.getElementById('avg-end-soc').textContent = socStats.average_end_soc.toFixed(1) + '%';
}

// Update provider statistics table
function updateProviderTable(providers) {
    const tableBody = document.querySelector('#provider-stats-table tbody');
    tableBody.innerHTML = ''; // Clear existing content
    
    // Make sure providers is an array and not null or undefined
    if (!Array.isArray(providers) || providers.length === 0) {
        const row = document.createElement('tr');
        row.innerHTML = `
            <td colspan="8" style="text-align: center;">No provider data available</td>
        `;
        tableBody.appendChild(row);
        return;
    }
    
    // Sort providers by total sessions (descending)
    const sortedProviders = [...providers].sort((a, b) => b.total_sessions - a.total_sessions);
    
    // Show all providers, including those with just 1 session
    const significantProviders = sortedProviders;
    
    significantProviders.forEach(provider => {
        const row = document.createElement('tr');
        
        // Determine success rate class
        let successRateClass = '';
        if (provider.success_rate >= 80) {
            successRateClass = 'success-high';
        } else if (provider.success_rate >= 50) {
            successRateClass = 'success-medium';
        } else {
            successRateClass = 'success-low';
        }
        
        // Format efficiency as N/A if it's zero (likely no data)
        const efficiencyDisplay = provider.avg_efficiency > 0 
            ? `${provider.avg_efficiency.toFixed(1)}%` 
            : 'N/A';
        
        // Format avg power as N/A if it's zero
        const avgPowerDisplay = provider.avg_power > 0 
            ? `${provider.avg_power.toFixed(1)}` 
            : 'N/A';
            
        row.innerHTML = `
            <td title="${provider.provider}">${provider.provider}</td>
            <td>${provider.total_sessions.toLocaleString()}</td>
            <td>${provider.successful_sessions.toLocaleString()}</td>
            <td>${provider.failed_sessions.toLocaleString()}</td>
            <td class="success-rate ${successRateClass}">${provider.success_rate.toFixed(1)}%</td>
            <td>${efficiencyDisplay}</td>
            <td>${provider.total_energy_added.toFixed(1)} kWh</td>
            <td>${avgPowerDisplay} kW</td>
        `;
        
        tableBody.appendChild(row);
    });
    
    // If there are no providers with sufficient data
    if (significantProviders.length === 0) {
        const row = document.createElement('tr');
        row.innerHTML = `
            <td colspan="8" style="text-align: center;">No provider data available</td>
        `;
        tableBody.appendChild(row);
    }
    
    // Add a total row at the bottom
    const totalRow = document.createElement('tr');
    totalRow.classList.add('total-row');
    
    // Calculate totals
    const totalSessions = providers.reduce((sum, p) => sum + p.total_sessions, 0);
    const successfulSessions = providers.reduce((sum, p) => sum + p.successful_sessions, 0);
    const failedSessions = providers.reduce((sum, p) => sum + p.failed_sessions, 0);
    const totalEnergy = providers.reduce((sum, p) => sum + p.total_energy_added, 0);
    const successRate = totalSessions > 0 ? (successfulSessions / totalSessions) * 100 : 0;
    
    let totalRateClass = '';
    if (successRate >= 80) {
        totalRateClass = 'success-high';
    } else if (successRate >= 50) {
        totalRateClass = 'success-medium';
    } else {
        totalRateClass = 'success-low';
    }
    
    totalRow.innerHTML = `
        <td><strong>TOTAL</strong></td>
        <td><strong>${totalSessions.toLocaleString()}</strong></td>
        <td><strong>${successfulSessions.toLocaleString()}</strong></td>
        <td><strong>${failedSessions.toLocaleString()}</strong></td>
        <td class="success-rate ${totalRateClass}"><strong>${successRate.toFixed(1)}%</strong></td>
        <td>-</td>
        <td><strong>${totalEnergy.toFixed(1)} kWh</strong></td>
        <td>-</td>
    `;
    
    tableBody.appendChild(totalRow);
}

// Create provider failure rate chart
function createProviderFailureChart(providers) {
    // Check if we have valid data
    if (!Array.isArray(providers) || providers.length === 0) {
        // Display a message in the chart container
        const chartContainer = document.getElementById('provider-failure-chart');
        chartContainer.innerHTML = '<div class="no-data-message">No provider data available</div>';
        return;
    }
    
    // Filter to providers with at least 5 sessions for meaningful data
    // and at least 1 failed session
    const significantProviders = providers.filter(p => 
        p.total_sessions >= 5 && p.failed_sessions > 0);
    
    if (significantProviders.length === 0) {
        // Display a message in the chart container
        const chartContainer = document.getElementById('provider-failure-chart');
        chartContainer.innerHTML = '<div class="no-data-message">No providers with failures available</div>';
        return;
    }
    
    // Calculate failure rates for each provider
    significantProviders.forEach(p => {
        p.failure_rate = 100 - p.success_rate;
    });
    
    // Sort by failure rate (descending) - highest failure rate first
    const sortedProviders = [...significantProviders].sort((a, b) => b.failure_rate - a.failure_rate);
    
    // Limit to top 10 providers with highest failure rates
    const topProviders = sortedProviders.slice(0, 10);
    
    // Use original provider names for display
    const providerNames = topProviders.map(p => p.provider);
    const failureRates = topProviders.map(p => p.failure_rate);
    const sessionCounts = topProviders.map(p => p.total_sessions);
    const failedCounts = topProviders.map(p => p.failed_sessions);
    
    const trace = {
        x: providerNames,
        y: failureRates,
        type: 'bar',
        name: 'Failure Rate (%)',
        marker: {
            color: failureRates.map(rate => {
                if (rate >= 50) return '#e74c3c';  // High failure - red
                if (rate >= 20) return '#f39c12';  // Medium failure - orange
                return '#2ecc71';                  // Low failure - green
            }),
            line: {
                width: 1,
                color: '#888'
            }
        },
        text: failureRates.map((rate, i) => 
            `${rate.toFixed(1)}%<br>(${failedCounts[i]}/${sessionCounts[i]})`),
        textposition: 'outside',
        textfont: {
            size: 11,
            color: '#333'
        },
        hovertemplate: '<b>%{x}</b><br>Failure Rate: %{y:.1f}%<br>Failed: %{text}<extra></extra>',
        width: 0.6 // Make bars narrower
    };
    
    const layout = {
        title: {
            text: 'Charging Failure Rates by Provider',
            font: { size: 20 }
        },
        xaxis: {
            title: 'Provider',
            tickangle: -45,
            tickfont: {
                size: 11
            },
            automargin: true // Automatically adjust margin to fit labels
        },
        yaxis: {
            title: 'Failure Rate (%)',
            range: [0, 110], // Fixed range with some space at top for labels
            fixedrange: true, // Prevent users from zooming/panning
            ticksuffix: '%'
        },
        height: 550, // Taller chart
        autosize: true,
        margin: { 
            b: 120, // Ample bottom margin for provider labels
            t: 80,  // Top margin for title
            l: 70,  // Left margin for y-axis labels
            r: 50   // Right margin for values that might extend beyond bars
        },
        bargap: 0.4, // More space between bars
        template: plotlyTemplate
    };
    
    // Create responsive chart
    const config = {
        responsive: true,
        displayModeBar: false // Hide the mode bar
    };
    
    Plotly.newPlot('provider-failure-chart', [trace], layout, config);
}

// Create session outcomes pie chart
function createSessionOutcomesChart(globalStats) {
    // Check if we have valid data
    if (!globalStats || typeof globalStats !== 'object' || 
        typeof globalStats.successful_sessions !== 'number' || 
        typeof globalStats.failed_sessions !== 'number') {
        // Display a message in the chart container
        const chartContainer = document.getElementById('session-outcomes-chart');
        chartContainer.innerHTML = '<div class="no-data-message">No session data available</div>';
        return;
    }
    
    const total = globalStats.successful_sessions + globalStats.failed_sessions;
    
    if (total === 0) {
        // Display a message if there are no sessions
        const chartContainer = document.getElementById('session-outcomes-chart');
        chartContainer.innerHTML = '<div class="no-data-message">No session data available</div>';
        return;
    }
    
    const successPercent = ((globalStats.successful_sessions / total) * 100).toFixed(1);
    const failedPercent = ((globalStats.failed_sessions / total) * 100).toFixed(1);
    
    const data = [{
        values: [globalStats.successful_sessions, globalStats.failed_sessions],
        labels: ['Successful', 'Failed'],
        type: 'pie',
        marker: {
            colors: ['#2ecc71', '#e74c3c'],
            line: {
                color: '#fff',
                width: 2
            }
        },
        textinfo: 'label+percent',
        textposition: 'outside',
        textfont: {
            size: 14,
            color: '#333'
        },
        hoverinfo: 'label+value+percent',
        hole: 0.4, // Create a donut chart for better visual appeal
        pull: [0.03, 0], // Slightly separate the success slice
        insidetextorientation: 'horizontal'
    }];
    
    const layout = {
        title: {
            text: 'Charging Session Outcomes',
            font: { size: 20 }
        },
        annotations: [
            {
                // Add total sessions in the center of the donut
                text: `<b>${total}</b><br>Sessions`,
                x: 0.5,
                y: 0.5,
                font: {
                    size: 16
                },
                showarrow: false
            }
        ],
        height: 450,
        autosize: true,
        margin: {
            t: 80,
            b: 80,
            l: 40,
            r: 40
        },
        showlegend: true,
        legend: {
            orientation: 'h',
            xanchor: 'center',
            yanchor: 'top',
            y: -0.15,
            x: 0.5
        },
        template: plotlyTemplate
    };
    
    // Create responsive chart
    const config = {
        responsive: true,
        displayModeBar: false // Hide the mode bar
    };
    
    Plotly.newPlot('session-outcomes-chart', data, layout, config);
}

// Create energy by provider chart
function createEnergyByProviderChart(providers) {
    // Check if we have valid data
    if (!Array.isArray(providers) || providers.length === 0) {
        // Display a message in the chart container
        const chartContainer = document.getElementById('energy-by-provider-chart');
        chartContainer.innerHTML = '<div class="no-data-message">No provider data available</div>';
        return;
    }
    
    // Filter to providers with at least some energy added
    const providersWithEnergy = providers.filter(p => p.total_energy_added > 0);
    
    if (providersWithEnergy.length === 0) {
        // Display a message in the chart container
        const chartContainer = document.getElementById('energy-by-provider-chart');
        chartContainer.innerHTML = '<div class="no-data-message">No energy data available for providers</div>';
        return;
    }
    
    // Sort by total energy added (descending)
    const sortedProviders = [...providersWithEnergy].sort((a, b) => b.total_energy_added - a.total_energy_added);
    
    // Limit to top 12 providers for better readability
    const topProviders = sortedProviders.slice(0, 12);
    
    // Use original provider names
    const providerNames = topProviders.map(p => p.provider);
    const energyValues = topProviders.map(p => p.total_energy_added);
    const sessionCounts = topProviders.map(p => p.total_sessions);
    
    // Find max energy to properly scale the y-axis
    const maxEnergy = Math.max(...energyValues);
    const yAxisMax = Math.ceil(maxEnergy * 1.2); // Add 20% padding for labels
    
    const trace = {
        x: providerNames,
        y: energyValues,
        type: 'bar',
        name: 'Energy Added (kWh)',
        marker: {
            color: '#3498db',
            opacity: 0.8,
            line: {
                width: 1,
                color: '#2980b9'
            }
        },
        text: energyValues.map((val, i) => `${val.toFixed(1)} kWh<br>(${sessionCounts[i]} sessions)`),
        textposition: 'outside',
        textfont: {
            size: 11,
            color: '#333'
        },
        hovertemplate: '<b>%{x}</b><br>Total Energy: %{y:.1f} kWh<br>Sessions: %{text}<extra></extra>',
        width: 0.6 // Make bars narrower
    };
    
    const layout = {
        title: {
            text: 'Total Energy Added by Provider (kWh)',
            font: { size: 20 }
        },
        xaxis: {
            title: 'Provider',
            tickangle: -45,
            tickfont: {
                size: 11
            },
            automargin: true // Automatically adjust margin to fit labels
        },
        yaxis: {
            title: 'Energy Added (kWh)',
            rangemode: 'tozero',
            range: [0, yAxisMax], // Set fixed range with padding for labels
            fixedrange: true,
            ticksuffix: ' kWh'
        },
        height: 580, // Taller chart for better readability
        autosize: true, 
        margin: { 
            b: 120, // Ample bottom margin for provider labels
            t: 80,  // Top margin for title
            l: 80,  // Left margin for y-axis labels and values
            r: 60   // Right margin for values that might extend beyond bars
        },
        bargap: 0.4, // More space between bars
        template: plotlyTemplate
    };
    
    // Create responsive chart
    const config = {
        responsive: true,
        displayModeBar: false // Hide the mode bar
    };
    
    Plotly.newPlot('energy-by-provider-chart', [trace], layout, config);
}

// Display error message
function showErrorMessage(message) {
    // Create an error message element
    const errorElement = document.createElement('div');
    errorElement.className = 'error-message';
    errorElement.style.backgroundColor = '#ffebee';
    errorElement.style.color = '#c62828';
    errorElement.style.padding = '15px';
    errorElement.style.borderRadius = '5px';
    errorElement.style.marginBottom = '20px';
    errorElement.style.textAlign = 'center';
    errorElement.textContent = message;
    
    // Insert at the top of the container
    const container = document.querySelector('.container');
    container.insertBefore(errorElement, container.firstChild.nextSibling);
}

// We removed the formatProviderName function as we want to display the original provider names exactly as stored in the database
