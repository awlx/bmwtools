// App version - should match the server version in pkg/api/handler.go
const APP_VERSION = '1.0.0';

// Global variables
let sessions = [];
let currentSession = null;
let useMiles = false;
let chargingLocationsMap = null;

// Define a modern Plotly template with enhanced styling
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
            zerolinecolor: '#e0e0e0',
            tickfont: {
                family: 'SF Pro Display, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif',
                size: 12,
                color: '#555'
            }
        },
        yaxis: {
            gridcolor: '#f0f0f0',
            zerolinecolor: '#e0e0e0',
            tickfont: {
                family: 'SF Pro Display, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif',
                size: 12,
                color: '#555'
            }
        }
    }
};

// DOM elements
const disclaimerEl = document.getElementById('disclaimer');
const energyWarningEl = document.getElementById('energy-data-warning');
const fileUploadEl = document.getElementById('upload-json');
const loadDemoBtn = document.getElementById('load-demo-data');
const startDateEl = document.getElementById('start-date');
const endDateEl = document.getElementById('end-date');
const applyDateFilterBtn = document.getElementById('apply-date-filter');
const toggleUnitsBtn = document.getElementById('toggle-units');
const sessionDropdownEl = document.getElementById('session-dropdown');

// Initialize the application
function init() {
    setDisclaimer();
    setupEventListeners();
}

// Set up the disclaimer
function setDisclaimer() {
    disclaimerEl.textContent = 'Disclaimer: This application stores all uploaded data in memory, if you refresh your session is lost.\n' +
        'CarData contains location data of your charges. Use at your own risk!\n' +
        'You can verify authenticity at https://github.com/awlx/bmwtools';
}

// Set up event listeners
function setupEventListeners() {
    fileUploadEl.addEventListener('change', handleFileUpload);
    loadDemoBtn.addEventListener('click', loadDemoData);
    applyDateFilterBtn.addEventListener('click', applyDateFilter);
    toggleUnitsBtn.addEventListener('click', toggleUnits);
    sessionDropdownEl.addEventListener('change', handleSessionSelection);
    
    // Set up drag and drop events for the upload container
    const uploadContainer = document.querySelector('.upload-container');
    const fileLabel = document.querySelector('.file-label');
    
    if (uploadContainer && fileLabel) {
        // Prevent default behavior to allow drop
        ['dragenter', 'dragover', 'dragleave', 'drop'].forEach(eventName => {
            uploadContainer.addEventListener(eventName, preventDefaults, false);
        });
        
        // Add visual feedback for drag events
        ['dragenter', 'dragover'].forEach(eventName => {
            uploadContainer.addEventListener(eventName, () => {
                fileLabel.style.backgroundColor = '#e6f3ff';
                fileLabel.style.borderStyle = 'solid';
                fileLabel.style.borderColor = '#1f77b4';
            }, false);
        });
        
        ['dragleave', 'drop'].forEach(eventName => {
            uploadContainer.addEventListener(eventName, () => {
                fileLabel.style.backgroundColor = '';
                fileLabel.style.borderStyle = 'dashed';
                fileLabel.style.borderColor = '#1f77b4';
            }, false);
        });
        
        // Handle the dropped files
        uploadContainer.addEventListener('drop', handleDrop, false);
    }
    
    // Function to adjust the similarity threshold
    window.setProviderSimilarityThreshold = function(threshold) {
        if (threshold >= 0 && threshold <= 1) {
            window.providerSimilarityThreshold = threshold;
            console.log(`Provider similarity threshold set to ${threshold * 100}%`);
            return true;
        } else {
            console.error("Threshold must be between 0 and 1");
            return false;
        }
    };
}

// Prevent default drag behaviors
function preventDefaults(e) {
    e.preventDefault();
    e.stopPropagation();
}

// Handle drop event
async function handleDrop(event) {
    const dt = event.dataTransfer;
    const files = dt.files;
    
    if (files.length) {
        const file = files[0];
        // Use the same upload process as the file input
        await uploadFile(file);
    }
}

// Handle file upload
async function handleFileUpload(event) {
    const file = event.target.files[0];
    if (!file) return;
    await uploadFile(file);
}

// Generic file upload function
async function uploadFile(file) {
    // Check if file name contains the expected pattern
    if (!file.name.includes('BMW-CarData-')) {
        const proceed = confirm("The file doesn't match the expected pattern 'BMW-CarData-Ladehistorie_*'. Are you sure you want to upload it?");
        if (!proceed) return;
    }

    const formData = new FormData();
    formData.append('file', file);

    try {
        const response = await fetch('/api/upload', {
            method: 'POST',
            body: formData
        });

        if (!response.ok) {
            const errorData = await response.json();
            throw new Error(errorData.error || 'Error uploading file');
        }

        await loadSessionData();
    } catch (error) {
        console.error('Upload error:', error);
        alert(`Error: ${error.message}`);
    }
}

// Load demo data
async function loadDemoData() {
    try {
        const response = await fetch('/api/demo');
        if (!response.ok) {
            const errorData = await response.json();
            throw new Error(errorData.error || 'Error loading demo data');
        }

        await loadSessionData();
    } catch (error) {
        console.error('Demo data error:', error);
        alert(`Error: ${error.message}`);
    }
}

// Apply date filter
async function applyDateFilter() {
    await loadSessionData();
}

// Toggle between km and miles
function toggleUnits() {
    useMiles = !useMiles;
    updateCurrentKmGauge();
}

// Handle session selection
function handleSessionSelection() {
    const sessionIndex = sessionDropdownEl.value;
    if (sessionIndex !== null && sessionIndex !== '') {
        currentSession = sessions[parseInt(sessionIndex)];
        updateSessionDetails();
    }
}

// Load session data from API
async function loadSessionData() {
    try {
        // Prepare query parameters for date filter
        const startDate = startDateEl.value;
        const endDate = endDateEl.value;
        let dateParams = '';
        
        if (startDate && endDate) {
            dateParams = `?startDate=${startDate}&endDate=${endDate}`;
        }
        
        // Fetch sessions with date filter
        let sessionsUrl = '/api/sessions' + dateParams;
        const sessionsResponse = await fetch(sessionsUrl);
        if (!sessionsResponse.ok) {
            throw new Error('Failed to fetch sessions');
        }
        
        sessions = await sessionsResponse.json();

        // Fetch stats with the same date filter
        let statsUrl = '/api/stats' + dateParams;
        const statsResponse = await fetch(statsUrl);
        if (!statsResponse.ok) {
            throw new Error('Failed to fetch stats');
        }
        
        const stats = await statsResponse.json();

        // Update UI with fetched data
        updateSessionDropdown();
        updateDashboardVisualizations(stats);
        
        // Show warning if using estimated values
        if (stats.using_estimated_values) {
            energyWarningEl.textContent = "⚠️ Warning: Your JSON file is missing 'energyIncreaseHvbKwh' data. " +
                "Energy values are estimated using 98% efficiency for DC charging and 92% efficiency for AC charging.";
            energyWarningEl.style.display = 'block';
        } else {
            energyWarningEl.style.display = 'none';
        }

        // Initialize maps
        initChargingLocationsMap();
        
        // Select the first session by default
        if (sessions.length > 0) {
            sessionDropdownEl.value = "0";
            currentSession = sessions[0];
            updateSessionDetails();
        }

    } catch (error) {
        console.error('Error loading data:', error);
        alert(`Error: ${error.message}`);
    }
}

// Update session dropdown
function updateSessionDropdown() {
    sessionDropdownEl.innerHTML = '';
    
    sessions.forEach((session, index) => {
        const option = document.createElement('option');
        option.value = index;
        option.textContent = `${new Date(session.start_time).toLocaleString()} - ${session.location}`;
        sessionDropdownEl.appendChild(option);
    });
}

// Update dashboard visualizations
function updateDashboardVisualizations(stats) {
    // Create gauges
    createTotalEnergyGauge();
    updateCurrentKmGauge();
    createOverallEfficiencyGauge(stats.overall_efficiency);
    createPowerConsumptionGauge(stats.power_consumption_per_100km, stats.power_consumption_without_losses);
    createSessionStatsGauges(stats.session_stats);
    
    // Update providers lists
    updateProviderLists(stats.session_stats);
    
    // Update SOC stats
    updateSOCStats(stats.soc_stats);
    
    // Create overview charts
    createOverviewScatterplot();
    createAverageGridpowerScatterplot();
    createEstimatedBatteryCapacityScatterplot(stats.estimated_capacity);
}

// Update session details
function updateSessionDetails() {
    if (!currentSession) return;

    // Update session info text
    document.getElementById('session-info').textContent = 
        `Energy Added: ${currentSession.energy_added_hvb} kWh, Cost: €${currentSession.cost}, ` +
        `Efficiency: ${(currentSession.efficiency * 100).toFixed(2)}%, Location: ${currentSession.location}`;

    // Create visualizations for selected session
    createChargeDetailsGraph();
    createCombinedGauges();
    createGridPowerGraph();
    createRangeMap();
}

// Create modern total energy visualization
function createTotalEnergyGauge() {
    if (sessions.length === 0) return;

    const totalEnergyDc = sessions
        .filter(s => s.avg_power >= 12)
        .reduce((total, s) => total + s.energy_added_hvb, 0);
    
    const totalEnergyAc = sessions
        .filter(s => s.avg_power < 12)
        .reduce((total, s) => total + s.energy_added_hvb, 0);
    
    const totalEnergy = totalEnergyDc + totalEnergyAc;
    
    // Create a modern stacked bar chart with percentage breakdown
    const data = [
        {
            x: ['Total Energy'],
            y: [totalEnergyDc],
            name: 'DC Energy',
            type: 'bar',
            marker: {
                color: '#3498db',  // Blue for DC energy
                line: {
                    color: 'rgba(0,0,0,0.1)',
                    width: 1
                }
            },
            text: [`${totalEnergyDc.toFixed(1)} kWh (${((totalEnergyDc / totalEnergy) * 100).toFixed(0)}%)`],
            textposition: 'inside',
            insidetextfont: {
                color: 'white',
                size: 14
            },
            hovertemplate: 'DC Energy: %{y:.1f} kWh<br>%{text}<extra></extra>'
        },
        {
            x: ['Total Energy'],
            y: [totalEnergyAc],
            name: 'AC Energy',
            type: 'bar',
            marker: {
                color: '#2ecc71',  // Green for AC energy
                line: {
                    color: 'rgba(0,0,0,0.1)',
                    width: 1
                }
            },
            text: [`${totalEnergyAc.toFixed(1)} kWh (${((totalEnergyAc / totalEnergy) * 100).toFixed(0)}%)`],
            textposition: 'inside',
            insidetextfont: {
                color: 'white',
                size: 14
            },
            hovertemplate: 'AC Energy: %{y:.1f} kWh<br>%{text}<extra></extra>'
        }
    ];
    
    const layout = {
        title: {
            text: 'Total Energy Consumption',
            font: { size: 20, color: '#444', family: 'Roboto, sans-serif' }
        },
        barmode: 'stack',
        height: 280,
        margin: { t: 60, b: 40, l: 70, r: 40 },
        template: plotlyTemplate,
        yaxis: {
            title: {
                text: 'Energy (kWh)',
                font: { size: 14 }
            }
        },
        xaxis: {
            tickfont: { size: 14 }
        },
        legend: {
            orientation: 'h',
            yanchor: 'bottom',
            y: 1.02,
            xanchor: 'center',
            x: 0.5
        },
        annotations: [
            {
                x: 0,
                y: totalEnergy,
                text: `Total: ${totalEnergy.toFixed(1)} kWh`,
                showarrow: true,
                arrowhead: 0,
                arrowsize: 1,
                arrowwidth: 1,
                arrowcolor: '#666',
                ax: 0,
                ay: -40,
                font: {
                    size: 14,
                    color: '#444',
                    weight: 'bold'
                },
                bgcolor: 'rgba(255, 255, 255, 0.8)',
                bordercolor: '#ddd',
                borderwidth: 1,
                borderpad: 4,
                opacity: 0.9
            }
        ]
    };
    
    // Create the stacked bar chart
    Plotly.newPlot('total-energy-gauge', data, layout, {responsive: true});
    
    // Add additional information card below the chart
    const gaugeContainer = document.getElementById('total-energy-gauge');
    
    // Check if the info card already exists, remove if it does
    const existingInfo = gaugeContainer.querySelector('.energy-info-card');
    if (existingInfo) {
        existingInfo.remove();
    }
    
    // Create info card
    const infoCard = document.createElement('div');
    infoCard.className = 'energy-info-card';
    infoCard.style.display = 'flex';
    infoCard.style.justifyContent = 'space-around';
    infoCard.style.marginTop = '10px';
    infoCard.style.padding = '10px';
    infoCard.style.backgroundColor = '#f8f9fa';
    infoCard.style.borderRadius = '8px';
    infoCard.style.boxShadow = '0 2px 4px rgba(0,0,0,0.05)';
    
    // Create energy stats
    const createEnergyStat = (label, value, color, icon) => {
        const stat = document.createElement('div');
        stat.style.textAlign = 'center';
        stat.style.padding = '10px';
        
        const iconEl = document.createElement('div');
        iconEl.textContent = icon;
        iconEl.style.fontSize = '24px';
        iconEl.style.marginBottom = '5px';
        
        const valueEl = document.createElement('div');
        valueEl.textContent = value;
        valueEl.style.fontSize = '18px';
        valueEl.style.fontWeight = 'bold';
        valueEl.style.color = color;
        
        const labelEl = document.createElement('div');
        labelEl.textContent = label;
        labelEl.style.fontSize = '12px';
        labelEl.style.color = '#777';
        
        stat.appendChild(iconEl);
        stat.appendChild(valueEl);
        stat.appendChild(labelEl);
        
        return stat;
    };
    
    // Calculate averages and other interesting stats
    const avgEnergyPerSession = sessions.length > 0 ? 
        (totalEnergy / sessions.length).toFixed(1) + ' kWh' : '0 kWh';
    
    const dcPercentage = totalEnergy > 0 ? 
        ((totalEnergyDc / totalEnergy) * 100).toFixed(0) + '%' : '0%';
    
    const acPercentage = totalEnergy > 0 ? 
        ((totalEnergyAc / totalEnergy) * 100).toFixed(0) + '%' : '0%';
    
    // Add the stats to the card
    infoCard.appendChild(createEnergyStat('Total Energy', totalEnergy.toFixed(1) + ' kWh', '#9c27b0', '⚡'));
    infoCard.appendChild(createEnergyStat('Avg per Session', avgEnergyPerSession, '#2196f3', '📊'));
    infoCard.appendChild(createEnergyStat('DC Charging', dcPercentage, '#3498db', '🔋'));
    infoCard.appendChild(createEnergyStat('AC Charging', acPercentage, '#2ecc71', '🔌'));
    
    // Add the info card to the container
    gaugeContainer.appendChild(infoCard);
}

// Update current km gauge with a modern card visualization
function updateCurrentKmGauge() {
    if (sessions.length === 0) return;
    
    const startDate = startDateEl.value;
    const endDate = endDateEl.value;
    let distance, firstDate, lastDate, numSessions;
    
    if (startDate && endDate) {
        const filteredSessions = sessions.filter(s => {
            const sessionDate = new Date(s.start_time);
            return sessionDate >= new Date(startDate) && sessionDate <= new Date(endDate);
        });
        
        if (filteredSessions.length > 0) {
            distance = Math.max(...filteredSessions.map(s => s.mileage)) - 
                      Math.min(...filteredSessions.map(s => s.mileage));
            
            // Get first and last date for the date range display
            const sortedSessions = [...filteredSessions].sort((a, b) => 
                new Date(a.start_time) - new Date(b.start_time)
            );
            firstDate = new Date(sortedSessions[0].start_time);
            lastDate = new Date(sortedSessions[sortedSessions.length - 1].start_time);
            numSessions = filteredSessions.length;
        } else {
            distance = 0;
            firstDate = new Date(startDate);
            lastDate = new Date(endDate);
            numSessions = 0;
        }
    } else {
        distance = Math.max(...sessions.map(s => s.mileage));
        // Get first and last date from all sessions
        const sortedSessions = [...sessions].sort((a, b) => 
            new Date(a.start_time) - new Date(b.start_time)
        );
        firstDate = new Date(sortedSessions[0].start_time);
        lastDate = new Date(sortedSessions[sortedSessions.length - 1].start_time);
        numSessions = sessions.length;
    }
    
    // Convert to miles if needed
    const displayDistance = useMiles ? distance * 0.621371 : distance;
    const unitLabel = useMiles ? "miles" : "km";
    
    // Calculate days between first and last session
    const dayDiff = Math.ceil((lastDate - firstDate) / (1000 * 60 * 60 * 24));
    
    // Create a modern card visual instead of a gauge
    const kmGaugeElement = document.getElementById('current-km-gauge');
    kmGaugeElement.innerHTML = '';
    
    // Create the card container
    const card = document.createElement('div');
    card.className = 'distance-card';
    card.style.height = '100%';
    card.style.display = 'flex';
    card.style.flexDirection = 'column';
    card.style.justifyContent = 'center';
    card.style.alignItems = 'center';
    card.style.padding = '20px';
    card.style.backgroundColor = '#ffffff';
    card.style.borderRadius = '10px';
    card.style.boxShadow = '0 4px 12px rgba(0,0,0,0.08)';
    card.style.position = 'relative';
    card.style.overflow = 'hidden';
    card.style.transition = 'transform 0.3s ease';
    
    // Add hover effect
    card.onmouseover = () => card.style.transform = 'translateY(-5px)';
    card.onmouseout = () => card.style.transform = 'translateY(0)';
    
    // Create the header section
    const header = document.createElement('div');
    header.style.width = '100%';
    header.style.borderBottom = '1px solid #f0f0f0';
    header.style.paddingBottom = '15px';
    header.style.marginBottom = '15px';
    header.style.textAlign = 'center';
    
    const title = document.createElement('h3');
    title.textContent = 'Distance Covered';
    title.style.margin = '0';
    title.style.color = '#444';
    title.style.fontSize = '18px';
    header.appendChild(title);
    
    // Create the main distance display
    const distanceContainer = document.createElement('div');
    distanceContainer.style.display = 'flex';
    distanceContainer.style.flexDirection = 'column';
    distanceContainer.style.alignItems = 'center';
    distanceContainer.style.margin = '20px 0';
    
    const distanceValue = document.createElement('div');
    distanceValue.style.fontSize = '56px';
    distanceValue.style.fontWeight = '700';
    distanceValue.style.color = '#1f77b4';
    distanceValue.style.lineHeight = '1';
    distanceValue.style.background = 'linear-gradient(45deg, #1f77b4, #2ca8ff)';
    distanceValue.style.WebkitBackgroundClip = 'text';
    distanceValue.style.WebkitTextFillColor = 'transparent';
    distanceValue.textContent = Math.round(displayDistance).toLocaleString();
    
    const distanceUnit = document.createElement('div');
    distanceUnit.style.fontSize = '18px';
    distanceUnit.style.color = '#777';
    distanceUnit.style.marginTop = '5px';
    distanceUnit.textContent = unitLabel;
    
    distanceContainer.appendChild(distanceValue);
    distanceContainer.appendChild(distanceUnit);
    
    // Create additional stats container
    const statsContainer = document.createElement('div');
    statsContainer.style.display = 'grid';
    statsContainer.style.gridTemplateColumns = 'repeat(2, 1fr)';
    statsContainer.style.gap = '15px';
    statsContainer.style.width = '100%';
    statsContainer.style.marginTop = '10px';
    
    // Function to create a stat item
    function createStatItem(label, value, icon) {
        const stat = document.createElement('div');
        stat.style.display = 'flex';
        stat.style.flexDirection = 'column';
        stat.style.alignItems = 'center';
        stat.style.backgroundColor = '#f8f9fa';
        stat.style.padding = '12px';
        stat.style.borderRadius = '8px';
        
        const iconElement = document.createElement('div');
        iconElement.textContent = icon;
        iconElement.style.fontSize = '20px';
        iconElement.style.marginBottom = '5px';
        
        const valueElement = document.createElement('div');
        valueElement.textContent = value;
        valueElement.style.fontSize = '18px';
        valueElement.style.fontWeight = '600';
        valueElement.style.color = '#444';
        
        const labelElement = document.createElement('div');
        labelElement.textContent = label;
        labelElement.style.fontSize = '12px';
        labelElement.style.color = '#777';
        
        stat.appendChild(iconElement);
        stat.appendChild(valueElement);
        stat.appendChild(labelElement);
        
        return stat;
    }
    
    // Add stat items
    statsContainer.appendChild(createStatItem('Time Period', dayDiff + ' days', '📅'));
    statsContainer.appendChild(createStatItem('Sessions', numSessions, '🔌'));
    
    // Add date range info
    const dateRange = document.createElement('div');
    dateRange.style.marginTop = '20px';
    dateRange.style.fontSize = '13px';
    dateRange.style.color = '#666';
    dateRange.style.textAlign = 'center';
    dateRange.style.width = '100%';
    dateRange.textContent = `${firstDate.toLocaleDateString()} - ${lastDate.toLocaleDateString()}`;
    
    // Assemble the card
    card.appendChild(header);
    card.appendChild(distanceContainer);
    card.appendChild(statsContainer);
    card.appendChild(dateRange);
    
    // Add the unit toggle button at the bottom
    const toggleButton = document.createElement('button');
    toggleButton.textContent = useMiles ? 'Switch to KM' : 'Switch to Miles';
    toggleButton.style.marginTop = '20px';
    toggleButton.style.padding = '8px 16px';
    toggleButton.style.backgroundColor = '#1f77b4';
    toggleButton.style.color = 'white';
    toggleButton.style.border = 'none';
    toggleButton.style.borderRadius = '5px';
    toggleButton.style.cursor = 'pointer';
    toggleButton.style.fontSize = '14px';
    toggleButton.style.transition = 'background-color 0.2s';
    toggleButton.addEventListener('mouseenter', () => { toggleButton.style.backgroundColor = '#155d8e'; });
    toggleButton.addEventListener('mouseleave', () => { toggleButton.style.backgroundColor = '#1f77b4'; });
    toggleButton.addEventListener('click', toggleUnits);
    
    card.appendChild(toggleButton);
    
    // Add card to the container
    kmGaugeElement.appendChild(card);
}

// Create a modern overall efficiency visualization
function createOverallEfficiencyGauge(efficiency) {
    // Calculate the efficiency percentage
    const efficiencyPercentage = efficiency * 100;
    
    // Create a custom radial gauge with gradient
    const gaugeContainer = document.getElementById('overall-efficiency-gauge');
    gaugeContainer.innerHTML = '';
    
    // Create a modern circular gauge visualization
    const card = document.createElement('div');
    card.style.height = '100%';
    card.style.display = 'flex';
    card.style.flexDirection = 'column';
    card.style.justifyContent = 'center';
    card.style.alignItems = 'center';
    card.style.backgroundColor = '#ffffff';
    card.style.borderRadius = '10px';
    card.style.boxShadow = '0 4px 12px rgba(0,0,0,0.08)';
    card.style.position = 'relative';
    card.style.padding = '20px';
    card.style.transition = 'transform 0.3s ease';
    
    // Add hover effect
    card.onmouseover = () => card.style.transform = 'translateY(-5px)';
    card.onmouseout = () => card.style.transform = 'translateY(0)';
    
    // Create title
    const title = document.createElement('h3');
    title.textContent = 'Overall Charging Efficiency';
    title.style.margin = '0 0 20px 0';
    title.style.color = '#444';
    title.style.fontSize = '18px';
    title.style.textAlign = 'center';
    
    // Create gauge container
    const gauge = document.createElement('div');
    gauge.style.position = 'relative';
    gauge.style.width = '200px';
    gauge.style.height = '200px';
    gauge.style.display = 'flex';
    gauge.style.justifyContent = 'center';
    gauge.style.alignItems = 'center';
    
    // Determine color based on efficiency
    let gaugeColor = '#FF5E5E'; // Red for low efficiency
    if (efficiencyPercentage >= 85) {
        gaugeColor = '#4CAF50'; // Green for high efficiency
    } else if (efficiencyPercentage >= 70) {
        gaugeColor = '#FFC107'; // Yellow for medium efficiency
    } else if (efficiencyPercentage >= 50) {
        gaugeColor = '#FF9800'; // Orange for below average
    }
    
    // Create SVG for circular gauge
    const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
    svg.setAttribute('viewBox', '0 0 100 100');
    svg.style.width = '100%';
    svg.style.height = '100%';
    
    // Create background circle
    const backgroundCircle = document.createElementNS('http://www.w3.org/2000/svg', 'circle');
    backgroundCircle.setAttribute('cx', '50');
    backgroundCircle.setAttribute('cy', '50');
    backgroundCircle.setAttribute('r', '45');
    backgroundCircle.setAttribute('fill', 'none');
    backgroundCircle.setAttribute('stroke', '#f0f0f0');
    backgroundCircle.setAttribute('stroke-width', '10');
    
    // Add gradient definition
    const defs = document.createElementNS('http://www.w3.org/2000/svg', 'defs');
    const linearGradient = document.createElementNS('http://www.w3.org/2000/svg', 'linearGradient');
    linearGradient.setAttribute('id', 'efficiencyGradient');
    linearGradient.setAttribute('x1', '0%');
    linearGradient.setAttribute('y1', '0%');
    linearGradient.setAttribute('x2', '100%');
    linearGradient.setAttribute('y2', '100%');
    
    const stop1 = document.createElementNS('http://www.w3.org/2000/svg', 'stop');
    stop1.setAttribute('offset', '0%');
    stop1.setAttribute('stop-color', gaugeColor);
    
    const stop2 = document.createElementNS('http://www.w3.org/2000/svg', 'stop');
    stop2.setAttribute('offset', '100%');
    stop2.setAttribute('stop-color', shadeColor(gaugeColor, 30)); // Darker shade
    
    linearGradient.appendChild(stop1);
    linearGradient.appendChild(stop2);
    defs.appendChild(linearGradient);
    svg.appendChild(defs);
    
    // Calculate circle values
    const radius = 45;
    const circumference = 2 * Math.PI * radius;
    const progress = (100 - efficiencyPercentage) / 100 * circumference;
    
    // Create progress circle
    const progressCircle = document.createElementNS('http://www.w3.org/2000/svg', 'circle');
    progressCircle.setAttribute('cx', '50');
    progressCircle.setAttribute('cy', '50');
    progressCircle.setAttribute('r', '45');
    progressCircle.setAttribute('fill', 'none');
    progressCircle.setAttribute('stroke', 'url(#efficiencyGradient)');
    progressCircle.setAttribute('stroke-width', '10');
    progressCircle.setAttribute('stroke-dasharray', circumference);
    progressCircle.setAttribute('stroke-dashoffset', progress);
    progressCircle.setAttribute('transform', 'rotate(-90, 50, 50)');
    progressCircle.style.transition = 'stroke-dashoffset 1s ease-in-out';
    
    svg.appendChild(backgroundCircle);
    svg.appendChild(progressCircle);
    gauge.appendChild(svg);
    
    // Add efficiency value in center
    const valueContainer = document.createElement('div');
    valueContainer.style.position = 'absolute';
    valueContainer.style.display = 'flex';
    valueContainer.style.flexDirection = 'column';
    valueContainer.style.alignItems = 'center';
    valueContainer.style.justifyContent = 'center';
    
    const value = document.createElement('div');
    value.textContent = efficiencyPercentage.toFixed(1) + '%';
    value.style.fontSize = '32px';
    value.style.fontWeight = 'bold';
    value.style.color = gaugeColor;
    
    const label = document.createElement('div');
    label.textContent = 'Efficiency';
    label.style.fontSize = '14px';
    label.style.color = '#777';
    
    valueContainer.appendChild(value);
    valueContainer.appendChild(label);
    gauge.appendChild(valueContainer);
    
    // Create efficiency rating
    const ratingContainer = document.createElement('div');
    ratingContainer.style.marginTop = '20px';
    ratingContainer.style.textAlign = 'center';
    
    let ratingText = '';
    if (efficiencyPercentage >= 85) {
        ratingText = 'Excellent';
    } else if (efficiencyPercentage >= 75) {
        ratingText = 'Very Good';
    } else if (efficiencyPercentage >= 65) {
        ratingText = 'Good';
    } else if (efficiencyPercentage >= 55) {
        ratingText = 'Average';
    } else {
        ratingText = 'Below Average';
    }
    
    const rating = document.createElement('div');
    rating.textContent = ratingText;
    rating.style.fontWeight = 'bold';
    rating.style.fontSize = '18px';
    rating.style.color = gaugeColor;
    rating.style.marginBottom = '5px';
    
    const explanation = document.createElement('div');
    explanation.textContent = 'Energy transferred from grid to battery';
    explanation.style.fontSize = '13px';
    explanation.style.color = '#777';
    
    ratingContainer.appendChild(rating);
    ratingContainer.appendChild(explanation);
    
    // Assemble the card
    card.appendChild(title);
    card.appendChild(gauge);
    card.appendChild(ratingContainer);
    
    gaugeContainer.appendChild(card);
    
    // Helper function to darken a color
    function shadeColor(color, percent) {
        let R = parseInt(color.substring(1, 3), 16);
        let G = parseInt(color.substring(3, 5), 16);
        let B = parseInt(color.substring(5, 7), 16);

        R = Math.round(R * (100 - percent) / 100);
        G = Math.round(G * (100 - percent) / 100);
        B = Math.round(B * (100 - percent) / 100);

        R = (R < 255) ? R : 255;
        G = (G < 255) ? G : 255;
        B = (B < 255) ? B : 255;

        const RR = ((R.toString(16).length == 1) ? '0' + R.toString(16) : R.toString(16));
        const GG = ((G.toString(16).length == 1) ? '0' + G.toString(16) : G.toString(16));
        const BB = ((B.toString(16).length == 1) ? '0' + B.toString(16) : B.toString(16));

        return '#' + RR + GG + BB;
    }
}

// Create power consumption visualization with modern bar charts
function createPowerConsumptionGauge(consumption, consumptionWithoutLosses) {
    // Convert to a modern bar chart comparison
    const data = [
        {
            x: ['With Grid Losses', 'Without Grid Losses'],
            y: [consumption, consumptionWithoutLosses],
            type: 'bar',
            marker: {
                color: ['#6BD098', '#4A90E2'],
                line: {
                    color: 'rgba(0,0,0,0.1)',
                    width: 1
                }
            },
            text: [
                consumption.toFixed(1) + ' kWh/100km',
                consumptionWithoutLosses.toFixed(1) + ' kWh/100km'
            ],
            textposition: 'none',  // Hide the automatic text display to avoid overlap
            hoverinfo: 'y+text',
            hovertemplate: '%{y:.1f} kWh/100km<extra></extra>', // Custom hover template
            textfont: {
                size: 14,
                color: '#333'
            }
        }
    ];
    
    const layout = {
        title: {
            text: 'Power Consumption Comparison',
            font: { size: 20, color: '#444', family: 'Roboto, sans-serif' }
        },
        height: 300,
        margin: { t: 60, b: 80, l: 70, r: 40 },
        template: plotlyTemplate,
        yaxis: {
            title: {
                text: 'kWh/100km',
                font: { size: 14 }
            },
            range: [0, Math.max(consumption, consumptionWithoutLosses) * 1.2]
        },
        xaxis: {
            tickangle: 0,
            tickfont: { size: 14 }
        },
        annotations: [
            {
                x: 0,
                y: consumption,
                text: consumption.toFixed(1) + ' kWh/100km', // Include units in the annotation
                showarrow: false,
                yshift: 15, // Move above the bar instead of inside it
                font: { size: 14, color: '#333', weight: 'bold' }
            },
            {
                x: 1,
                y: consumptionWithoutLosses,
                text: consumptionWithoutLosses.toFixed(1) + ' kWh/100km', // Include units in the annotation
                showarrow: false,
                yshift: 15, // Move above the bar instead of inside it
                font: { size: 14, color: '#333', weight: 'bold' }
            }
        ]
    };
    
    // Calculate the percentage saved
    const savingsPercentage = ((consumption - consumptionWithoutLosses) / consumption * 100).toFixed(1);
    
    // Combine both gauges into a single visualization
    Plotly.newPlot('power-consumption-gauge', data, layout);
    
    // Add custom HTML to the second gauge container to display savings
    const efficiencyContainer = document.getElementById('power-consumption-without-grid-losses-gauge');
    efficiencyContainer.innerHTML = '';
    
    // Create a modern card to display the savings
    const savingsCard = document.createElement('div');
    savingsCard.style.height = '100%';
    savingsCard.style.display = 'flex';
    savingsCard.style.flexDirection = 'column';
    savingsCard.style.justifyContent = 'center';
    savingsCard.style.alignItems = 'center';
    savingsCard.style.backgroundColor = '#ffffff';
    savingsCard.style.borderRadius = '10px';
    savingsCard.style.boxShadow = '0 4px 12px rgba(0,0,0,0.08)';
    savingsCard.style.padding = '20px';
    savingsCard.style.transition = 'transform 0.3s ease';
    
    // Add hover effect
    savingsCard.onmouseover = () => savingsCard.style.transform = 'translateY(-5px)';
    savingsCard.onmouseout = () => savingsCard.style.transform = 'translateY(0)';
    
    // Create header
    const header = document.createElement('h3');
    header.textContent = 'Charging Energy Overhead';
    header.style.margin = '0 0 20px 0';
    header.style.color = '#444';
    header.style.fontSize = '18px';
    header.style.textAlign = 'center';
    
    // Create savings value display
    const savingsValueContainer = document.createElement('div');
    savingsValueContainer.style.textAlign = 'center';
    savingsValueContainer.style.marginBottom = '10px';
    
    const savingsValue = document.createElement('div');
    savingsValue.style.fontSize = '64px';
    savingsValue.style.fontWeight = 'bold';
    savingsValue.style.background = 'linear-gradient(45deg, #4A90E2, #6BD098)';
    savingsValue.style.WebkitBackgroundClip = 'text';
    savingsValue.style.WebkitTextFillColor = 'transparent';
    savingsValue.style.lineHeight = '1';
    savingsValue.textContent = savingsPercentage + '%';
    
    const savingsLabel = document.createElement('div');
    savingsLabel.textContent = 'Energy overhead from grid losses';
    savingsLabel.style.fontSize = '14px';
    savingsLabel.style.color = '#777';
    savingsLabel.style.marginTop = '10px';
    
    savingsValueContainer.appendChild(savingsValue);
    savingsValueContainer.appendChild(savingsLabel);
    
    // Create data comparison
    const comparisonContainer = document.createElement('div');
    comparisonContainer.style.marginTop = '20px';
    comparisonContainer.style.width = '100%';
    comparisonContainer.style.padding = '0 20px';
    
    const comparisonTable = document.createElement('table');
    comparisonTable.style.width = '100%';
    comparisonTable.style.borderCollapse = 'collapse';
    comparisonTable.style.fontSize = '14px';
    
    // Add rows to the comparison table
    const rows = [
        { label: 'With Grid Losses', value: consumption.toFixed(1) + ' kWh/100km', color: '#6BD098' },
        { label: 'Without Grid Losses', value: consumptionWithoutLosses.toFixed(1) + ' kWh/100km', color: '#4A90E2' },
        { label: 'Energy Overhead', value: (consumption - consumptionWithoutLosses).toFixed(1) + ' kWh/100km', color: '#FF9F43' }
    ];
    
    rows.forEach(row => {
        const tr = document.createElement('tr');
        tr.style.borderBottom = '1px solid #f0f0f0';
        
        const tdLabel = document.createElement('td');
        tdLabel.textContent = row.label;
        tdLabel.style.padding = '10px 0';
        tdLabel.style.color = '#555';
        
        const tdValue = document.createElement('td');
        tdValue.textContent = row.value;
        tdValue.style.padding = '10px 0';
        tdValue.style.textAlign = 'right';
        tdValue.style.fontWeight = 'bold';
        tdValue.style.color = row.color;
        
        tr.appendChild(tdLabel);
        tr.appendChild(tdValue);
        comparisonTable.appendChild(tr);
    });
    
    comparisonContainer.appendChild(comparisonTable);
    
    // Add a note about efficiency calculation
    const note = document.createElement('div');
    note.textContent = 'Note: Grid losses include AC/DC conversion inefficiency, heat generation, and energy used for preconditioning while charging';
    note.style.fontSize = '12px';
    note.style.color = '#999';
    note.style.marginTop = '20px';
    note.style.textAlign = 'center';
    
    // Assemble the card
    savingsCard.appendChild(header);
    savingsCard.appendChild(savingsValueContainer);
    savingsCard.appendChild(comparisonContainer);
    savingsCard.appendChild(note);
    
    efficiencyContainer.appendChild(savingsCard);
}

// Create modern session stats visualization
function createSessionStatsGauges(sessionStats) {
    const totalSessions = sessionStats.total_sessions;
    const failedSessions = sessionStats.total_failed_sessions;
    const successfulSessions = sessionStats.total_successful_sessions;
    const successRate = totalSessions > 0 ? (successfulSessions / totalSessions) * 100 : 0;
    
    // Create a pie chart for the total sessions container
    const data = [{
        values: [successfulSessions, failedSessions],
        labels: ['Successful', 'Failed'],
        type: 'pie',
        hole: 0.7,
        marker: {
            colors: ['#66BB6A', '#EF5350']
        },
        textinfo: 'percent',
        insidetextfont: {
            color: '#FFFFFF',
            size: 14
        },
        hoverinfo: 'label+value+percent',
        textposition: 'inside'
    }];
    
    const layout = {
        title: {
            text: 'Charging Sessions Overview',
            font: { size: 18, color: '#444' }
        },
        height: 300,
        showlegend: true,
        legend: {
            orientation: 'h',
            x: 0.5,
            y: -0.2,
            xanchor: 'center'
        },
        annotations: [{
            font: {
                size: 20,
                color: '#444'
            },
            showarrow: false,
            text: totalSessions,
            x: 0.5,
            y: 0.5
        },
        {
            font: {
                size: 14,
                color: '#777'
            },
            showarrow: false,
            text: 'TOTAL',
            x: 0.5,
            y: 0.38
        }],
        template: plotlyTemplate
    };
    
    Plotly.newPlot('total-sessions-gauge', data, layout);
    
    // Create modern cards for failed and successful sessions
    createSessionStatCard('failed-sessions-gauge', failedSessions, totalSessions, 'Failed Sessions', '#EF5350', '❌');
    createSessionStatCard('successful-sessions-gauge', successfulSessions, totalSessions, 'Successful Sessions', '#66BB6A', '✅');
    
    // Helper function to create a modern stat card
    function createSessionStatCard(elementId, value, total, title, color, icon) {
        const container = document.getElementById(elementId);
        container.innerHTML = '';
        
        // Create the card
        const card = document.createElement('div');
        card.style.height = '100%';
        card.style.backgroundColor = '#ffffff';
        card.style.borderRadius = '10px';
        card.style.boxShadow = '0 4px 12px rgba(0,0,0,0.08)';
        card.style.padding = '20px';
        card.style.display = 'flex';
        card.style.flexDirection = 'column';
        card.style.alignItems = 'center';
        card.style.justifyContent = 'center';
        card.style.transition = 'transform 0.3s ease';
        
        // Add hover effect
        card.onmouseover = () => card.style.transform = 'translateY(-5px)';
        card.onmouseout = () => card.style.transform = 'translateY(0)';
        
        // Create an icon for the card
        const iconElement = document.createElement('div');
        iconElement.textContent = icon;
        iconElement.style.fontSize = '36px';
        iconElement.style.marginBottom = '10px';
        
        // Create title
        const titleElement = document.createElement('h3');
        titleElement.textContent = title;
        titleElement.style.margin = '0 0 15px 0';
        titleElement.style.fontSize = '18px';
        titleElement.style.color = '#444';
        titleElement.style.textAlign = 'center';
        
        // Create value container
        const valueContainer = document.createElement('div');
        valueContainer.style.display = 'flex';
        valueContainer.style.flexDirection = 'column';
        valueContainer.style.alignItems = 'center';
        
        // Create main value
        const valueElement = document.createElement('div');
        valueElement.textContent = value;
        valueElement.style.fontSize = '48px';
        valueElement.style.fontWeight = 'bold';
        valueElement.style.color = color;
        valueElement.style.lineHeight = '1';
        
        // Create percentage
        const percentage = total > 0 ? Math.round((value / total) * 100) : 0;
        const percentageElement = document.createElement('div');
        percentageElement.textContent = `${percentage}% of total`;
        percentageElement.style.marginTop = '5px';
        percentageElement.style.fontSize = '14px';
        percentageElement.style.color = '#777';
        
        valueContainer.appendChild(valueElement);
        valueContainer.appendChild(percentageElement);
        
        // Create progress bar container
        const progressContainer = document.createElement('div');
        progressContainer.style.width = '100%';
        progressContainer.style.marginTop = '20px';
        
        // Create progress background
        const progressBackground = document.createElement('div');
        progressBackground.style.width = '100%';
        progressBackground.style.height = '6px';
        progressBackground.style.backgroundColor = '#f0f0f0';
        progressBackground.style.borderRadius = '3px';
        progressBackground.style.overflow = 'hidden';
        
        // Create progress fill
        const progressFill = document.createElement('div');
        progressFill.style.width = `${percentage}%`;
        progressFill.style.height = '100%';
        progressFill.style.backgroundColor = color;
        progressFill.style.borderRadius = '3px';
        progressFill.style.transition = 'width 1s ease-in-out';
        
        progressBackground.appendChild(progressFill);
        progressContainer.appendChild(progressBackground);
        
        // Assemble the card
        card.appendChild(iconElement);
        card.appendChild(titleElement);
        card.appendChild(valueContainer);
        card.appendChild(progressContainer);
        
        // Add more context based on the card type
        const contextElement = document.createElement('div');
        contextElement.style.marginTop = '20px';
        contextElement.style.fontSize = '13px';
        contextElement.style.color = '#777';
        contextElement.style.textAlign = 'center';
        
        if (title.includes('Failed')) {
            contextElement.textContent = 'Sessions where charging was interrupted or unsuccessful';
        } else {
            contextElement.textContent = 'Sessions completed successfully to desired charge level';
        }
        
        card.appendChild(contextElement);
        container.appendChild(card);
    }
}

// Update providers lists
async function updateProviderLists(sessionStats) {
    const topFailedProvidersEl = document.getElementById('top-failed-providers');
    const topSuccessfulProvidersEl = document.getElementById('top-successful-providers');
    
    // Apply consistent styling to provider lists
    const styleProviderList = (element, title) => {
        element.innerHTML = '';
        element.style.padding = '0';
        element.style.listStyle = 'none';
        
        // Add explanatory title
        if (title) {
            const titleEl = document.createElement('h4');
            titleEl.textContent = title;
            titleEl.style.margin = '10px 0';
            titleEl.style.fontSize = '16px';
            titleEl.style.color = '#333';
            titleEl.style.fontWeight = '600';
            element.appendChild(titleEl);
        }
    };
    
    styleProviderList(topFailedProvidersEl, "Top Providers with Failed Sessions");
    styleProviderList(topSuccessfulProvidersEl, "Top Providers with Successful Sessions");
    
    // Add titles to the top of the provider sections
    const addTopTitles = () => {
        // Remove any existing titles first to avoid duplicates
        const removeExistingTitles = (element) => {
            let previousSibling = element.previousElementSibling;
            if (previousSibling && previousSibling.tagName === 'H3') {
                previousSibling.remove();
            }
            // Also remove the note if it exists
            let nextSibling = element.nextElementSibling;
            if (nextSibling && nextSibling.classList.contains('threshold-note')) {
                nextSibling.remove();
            }
        };
        
        removeExistingTitles(topFailedProvidersEl);
        removeExistingTitles(topSuccessfulProvidersEl);
        
        // Create and add the top titles
        const failedTitle = document.createElement('h3');
        failedTitle.textContent = 'Top 5 Failed Providers';
        failedTitle.style.margin = '10px 0';
        failedTitle.style.color = '#333';
        failedTitle.style.fontWeight = '600';
        failedTitle.style.fontSize = '18px';
        failedTitle.style.textAlign = 'center';
        topFailedProvidersEl.parentNode.insertBefore(failedTitle, topFailedProvidersEl);
        
        const successTitle = document.createElement('h3');
        successTitle.textContent = 'Top 5 Successful Providers';
        successTitle.style.margin = '10px 0';
        successTitle.style.color = '#333';
        successTitle.style.fontWeight = '600';
        successTitle.style.fontSize = '18px';
        successTitle.style.textAlign = 'center';
        topSuccessfulProvidersEl.parentNode.insertBefore(successTitle, topSuccessfulProvidersEl);
        
        // Create and add note about 50+ sessions threshold
        const thresholdNote = document.createElement('div');
        thresholdNote.classList.add('threshold-note');
        thresholdNote.textContent = 'Note: Only providers with 50+ sessions are displayed in these charts';
        thresholdNote.style.fontSize = '12px';
        thresholdNote.style.fontStyle = 'italic';
        thresholdNote.style.color = '#666';
        thresholdNote.style.textAlign = 'center';
        thresholdNote.style.margin = '0 0 10px 0';
        
        // Add the note after each provider list
        const failedNote = thresholdNote.cloneNode(true);
        topFailedProvidersEl.parentNode.insertBefore(failedNote, topFailedProvidersEl.nextSibling);
        
        const successNote = thresholdNote.cloneNode(true);
        topSuccessfulProvidersEl.parentNode.insertBefore(successNote, topSuccessfulProvidersEl.nextSibling);
    };
    
    // Add the titles to the page
    addTopTitles();
    
    try {
        // Prepare date parameters for API call
        const startDate = startDateEl.value;
        const endDate = endDateEl.value;
        let dateParams = '';
        
        if (startDate && endDate) {
            dateParams = `?startDate=${startDate}&endDate=${endDate}`;
        }
        
        // Fetch grouped providers from backend API
        const groupedProvidersUrl = '/api/grouped-providers' + dateParams;
        const response = await fetch(groupedProvidersUrl);
        
        if (!response.ok) {
            throw new Error('Failed to fetch grouped providers');
        }
        
        const groupedProvidersData = await response.json();
        
        // Extract the lists from the response
        const groupedSuccessfulProviders = groupedProvidersData.grouped_successful_providers || [];
        const groupedFailedProviders = groupedProvidersData.grouped_failed_providers || [];
        const allProviders = groupedProvidersData.all_providers || [];
        
        // Create the all providers section
        if (allProviders && allProviders.length > 0) {
            createAllProvidersSection(allProviders);
        }
        
        // Helper function to create provider item for either failed or successful providers
        const createProviderItem = (provider, isFailedProvider) => {
            const name = provider.provider;
            const count = isFailedProvider ? provider.failed_count : provider.successful_count;
            const total = provider.total;
            const rate = isFailedProvider ? provider.failure_rate : provider.success_rate;
            
            // Create a container
            const li = document.createElement('div');
            li.style.padding = '10px';
            li.style.margin = '8px 0';
            li.style.backgroundColor = '#f5f5f5';
            li.style.borderRadius = '8px';
            li.style.boxShadow = '0 2px 4px rgba(0,0,0,0.1)';
            
            // Create header with provider name and count
            const header = document.createElement('div');
            header.style.display = 'flex';
            header.style.justifyContent = 'space-between';
            header.style.alignItems = 'center';
            header.style.marginBottom = '5px';
            
            // Add provider name
            const providerName = document.createElement('span');
            providerName.textContent = name;
            providerName.style.fontWeight = 'bold';
            providerName.style.color = '#333';
            providerName.style.fontSize = '14px';
            providerName.style.flexGrow = '1';
            
            // Add count badge
            const countBadge = document.createElement('span');
            const badgeText = isFailedProvider ? 
                `${count} failed sessions` : 
                `${count} successful sessions`;
            countBadge.textContent = badgeText;
            countBadge.style.backgroundColor = isFailedProvider ? '#ffcccc' : '#ccffcc';
            countBadge.style.color = isFailedProvider ? '#cc0000' : '#006600';
            countBadge.style.padding = '3px 8px';
            countBadge.style.borderRadius = '12px';
            countBadge.style.fontSize = '0.85em';
            countBadge.style.fontWeight = 'bold';
            
            header.appendChild(providerName);
            header.appendChild(countBadge);
            li.appendChild(header);
            
            // Create percentage bar
            const percentageContainer = document.createElement('div');
            percentageContainer.style.marginTop = '8px';
            
            // Add percentage info text
            const percentageInfo = document.createElement('div');
            percentageInfo.style.display = 'flex';
            percentageInfo.style.justifyContent = 'space-between';
            percentageInfo.style.marginBottom = '2px';
            percentageInfo.style.fontSize = '11px';
            percentageInfo.style.color = '#666';
            
            const totalSessionsText = document.createElement('span');
            totalSessionsText.textContent = `${total} total sessions`;
            
            const rateText = document.createElement('span');
            rateText.style.fontWeight = 'bold';
            rateText.textContent = isFailedProvider ? 
                `${rate.toFixed(1)}% failure rate` : 
                `${rate.toFixed(1)}% success rate`;
                
            percentageInfo.appendChild(totalSessionsText);
            percentageInfo.appendChild(rateText);
            percentageContainer.appendChild(percentageInfo);
            
            // Bar background
            const percentageBar = document.createElement('div');
            percentageBar.style.height = '6px';
            percentageBar.style.backgroundColor = '#e0e0e0';
            percentageBar.style.borderRadius = '3px';
            percentageBar.style.overflow = 'hidden';
            percentageBar.style.position = 'relative';
            
            // Bar fill
            const percentageFill = document.createElement('div');
            percentageFill.style.height = '100%';
            percentageFill.style.width = `${rate}%`;
            percentageFill.style.backgroundColor = isFailedProvider ? '#e74c3c' : '#2ecc71';
            percentageFill.style.borderRadius = '3px';
            percentageFill.style.position = 'absolute';
            percentageFill.style.left = '0';
            percentageFill.style.top = '0';
            
            percentageBar.appendChild(percentageFill);
            percentageContainer.appendChild(percentageBar);
            li.appendChild(percentageContainer);
            
            return li;
        };
        
        // Clear previous list items before adding new ones
        topFailedProvidersEl.innerHTML = '';
        topSuccessfulProvidersEl.innerHTML = '';
        
        // Create list items for failed providers (top 5)
        if (groupedFailedProviders.length > 0) {
            groupedFailedProviders.forEach(provider => {
                const li = createProviderItem(provider, true);
                topFailedProvidersEl.appendChild(li);
            });
        } else {
            // Add a message if there are no failed providers
            const noFailures = document.createElement('div');
            noFailures.textContent = 'No failed sessions data available';
            noFailures.style.padding = '10px';
            noFailures.style.textAlign = 'center';
            noFailures.style.color = '#666';
            topFailedProvidersEl.appendChild(noFailures);
        }
        
        // Create list items for successful providers (top 5)
        if (groupedSuccessfulProviders.length > 0) {
            groupedSuccessfulProviders.forEach(provider => {
                const li = createProviderItem(provider, false);
                topSuccessfulProvidersEl.appendChild(li);
            });
        } else {
            // Add a message if there are no successful providers
            const noSuccess = document.createElement('div');
            noSuccess.textContent = 'No successful sessions data available';
            noSuccess.style.padding = '10px';
            noSuccess.style.textAlign = 'center';
            noSuccess.style.color = '#666';
            topSuccessfulProvidersEl.appendChild(noSuccess);
        }
    } catch (error) {
        console.error('Error fetching grouped providers:', error);
        
        // Fall back to using original provider data from sessionStats
        const fallbackSuccessfulProviders = (sessionStats.top_successful_providers || []).map(p => ({
            provider: p.provider,
            successful_count: p.count,
            failed_count: 0,
            total: p.count,
            success_rate: 100,
            failure_rate: 0
        }));
        
        const fallbackFailedProviders = (sessionStats.top_failed_providers || []).map(p => ({
            provider: p.provider,
            successful_count: 0,
            failed_count: p.count,
            total: p.count,
            success_rate: 0,
            failure_rate: 100
        }));
        
        console.warn('Using fallback provider data from sessionStats');
        
        // Clear previous list items before adding new ones
        topFailedProvidersEl.innerHTML = '';
        topSuccessfulProvidersEl.innerHTML = '';
        
        // Create list items for failed providers using fallback data
        if (fallbackFailedProviders.length > 0) {
            fallbackFailedProviders.forEach(provider => {
                const li = createProviderItem(provider, true);
                topFailedProvidersEl.appendChild(li);
            });
        } else {
            // Add a message if there are no failed providers
            const noFailures = document.createElement('div');
            noFailures.textContent = 'No failed sessions data available';
            noFailures.style.padding = '10px';
            noFailures.style.textAlign = 'center';
            noFailures.style.color = '#666';
            topFailedProvidersEl.appendChild(noFailures);
        }
        
        // Create list items for successful providers using fallback data
        if (fallbackSuccessfulProviders.length > 0) {
            fallbackSuccessfulProviders.forEach(provider => {
                const li = createProviderItem(provider, false);
                topSuccessfulProvidersEl.appendChild(li);
            });
        } else {
            // Add a message if there are no successful providers
            const noSuccess = document.createElement('div');
            noSuccess.textContent = 'No successful sessions data available';
            noSuccess.style.padding = '10px';
            noSuccess.style.textAlign = 'center';
            noSuccess.style.color = '#666';
            topSuccessfulProvidersEl.appendChild(noSuccess);
        }
    }
    
    // Create a section to display debug info about unknown providers

    
    // Create a section to display all providers
    function createAllProvidersSection(allProviders) {
        // Find the container where we'll add the all providers section
        const container = document.querySelector('.container');
        if (!container) return;
        
        // Check if the section already exists and remove it if it does
        let existingSection = document.getElementById('all-providers-section');
        if (existingSection) {
            existingSection.remove();
        }
        
        // Create the section
        const section = document.createElement('div');
        section.id = 'all-providers-section';
        section.className = 'row';
        section.style.marginTop = '30px';
        section.style.marginBottom = '30px';
        section.style.padding = '15px';
        section.style.backgroundColor = '#f9f9f9';
        section.style.borderRadius = '8px';
        section.style.boxShadow = '0 2px 4px rgba(0,0,0,0.1)';
        
        // Create the header
        const header = document.createElement('div');
        header.className = 'col-12';
        header.innerHTML = `
            <h3 style="text-align: center; margin-bottom: 20px;">All Charging Providers</h3>
            <p style="text-align: center; margin-bottom: 20px;">Complete list of all providers, including those with fewer than 50 sessions</p>
        `;
        section.appendChild(header);
        
        // Create the table container
        const tableContainer = document.createElement('div');
        tableContainer.className = 'col-12';
        tableContainer.style.overflowX = 'auto';
        
        // Create the table
        const table = document.createElement('table');
        table.className = 'table table-striped table-hover';
        table.style.width = '100%';
        
        // Create table header
        const thead = document.createElement('thead');
        thead.innerHTML = `
            <tr>
                <th style="width: 40%">Provider Name</th>
                <th style="width: 15%">Total Sessions</th>
                <th style="width: 15%">Successful</th>
                <th style="width: 15%">Failed</th>
                <th style="width: 15%">Success Rate</th>
            </tr>
        `;
        table.appendChild(thead);
        
        // Create table body
        const tbody = document.createElement('tbody');
        
        // Sort providers by total sessions (descending)
        allProviders.sort((a, b) => b.total - a.total);
        
        // Add rows for each provider
        allProviders.forEach(provider => {
            const row = document.createElement('tr');
            
            // Provider name cell
            const nameCell = document.createElement('td');
            nameCell.textContent = provider.provider;
            nameCell.style.fontWeight = 'bold';
            
            // Total sessions cell
            const totalCell = document.createElement('td');
            totalCell.textContent = provider.total;
            
            // Successful sessions cell
            const successCell = document.createElement('td');
            successCell.textContent = provider.successful_count;
            successCell.style.color = '#2ecc71';
            
            // Failed sessions cell
            const failedCell = document.createElement('td');
            failedCell.textContent = provider.failed_count;
            failedCell.style.color = '#e74c3c';
            
            // Success rate cell
            const rateCell = document.createElement('td');
            const successRate = provider.success_rate;
            rateCell.textContent = `${successRate.toFixed(1)}%`;
            
            // Color the success rate based on percentage
            if (successRate >= 90) {
                rateCell.style.color = '#27ae60';  // Dark green
                rateCell.style.fontWeight = 'bold';
            } else if (successRate >= 75) {
                rateCell.style.color = '#2ecc71';  // Green
            } else if (successRate >= 50) {
                rateCell.style.color = '#f39c12';  // Orange
            } else {
                rateCell.style.color = '#e74c3c';  // Red
            }
            
            // Add cells to row
            row.appendChild(nameCell);
            row.appendChild(totalCell);
            row.appendChild(successCell);
            row.appendChild(failedCell);
            row.appendChild(rateCell);
            
            // Add row to table body
            tbody.appendChild(row);
        });
        
        table.appendChild(tbody);
        tableContainer.appendChild(table);
        section.appendChild(tableContainer);
        
        // Add to container
        container.appendChild(section);
    }
}

// Update SOC stats
// Create a progress bar for SOC visualization
function createSOCProgressBar(label, percentage, color) {
    const container = document.createElement('div');
    container.style.marginBottom = '8px';
    container.style.backgroundColor = '#ffffff';
    container.style.borderRadius = '6px';
    container.style.padding = '10px';
    container.style.boxShadow = '0 1px 3px rgba(0,0,0,0.1)';
    
    // Create header with label and percentage
    const header = document.createElement('div');
    header.style.display = 'flex';
    header.style.justifyContent = 'space-between';
    header.style.marginBottom = '5px';
    
    const labelEl = document.createElement('span');
    labelEl.textContent = label;
    labelEl.style.fontWeight = 'bold';
    labelEl.style.color = '#555555';
    labelEl.style.fontSize = '13px';
    
    // Handle undefined or null percentages safely
    const validPercentage = percentage !== undefined && percentage !== null ? percentage : 0;
    
    const percentageEl = document.createElement('span');
    percentageEl.textContent = `${validPercentage.toFixed(1)}%`;
    percentageEl.style.fontWeight = '600';
    percentageEl.style.color = color;
    percentageEl.style.fontSize = '13px';
    
    header.appendChild(labelEl);
    header.appendChild(percentageEl);
    
    // Create progress bar background
    const progressBarBg = document.createElement('div');
    progressBarBg.style.height = '10px';
    progressBarBg.style.backgroundColor = '#f0f0f0';
    progressBarBg.style.borderRadius = '5px';
    progressBarBg.style.overflow = 'hidden';
    
    // Create progress bar fill
    const progressBarFill = document.createElement('div');
    progressBarFill.style.height = '100%';
    progressBarFill.style.width = `${validPercentage}%`;
    progressBarFill.style.backgroundColor = color;
    progressBarFill.style.transition = 'width 1s ease-in-out';
    
    // Assemble progress bar
    progressBarBg.appendChild(progressBarFill);
    container.appendChild(header);
    container.appendChild(progressBarBg);
    
    return container;
}

function updateSOCStats(socStats) {
    const socStatsEl = document.getElementById('soc-stats');
    socStatsEl.innerHTML = '';
    
    // Create main container with side-by-side sections
    const mainContainer = document.createElement('div');
    mainContainer.style.display = 'grid';
    mainContainer.style.gridTemplateColumns = 'repeat(auto-fit, minmax(450px, 1fr))';
    mainContainer.style.gap = '20px';
    mainContainer.style.marginTop = '10px';
    
    // Left side: SOC Statistics cards
    const statsSection = document.createElement('div');
    statsSection.style.display = 'flex';
    statsSection.style.flexDirection = 'column';
    
    // Add stats title
    const statsTitle = document.createElement('h3');
    statsTitle.textContent = 'SOC Statistics';
    statsTitle.style.margin = '0 0 10px 0';
    statsTitle.style.color = '#333';
    statsSection.appendChild(statsTitle);
    
    // Create a card container for count stats - make it more compact
    const cardContainer = document.createElement('div');
    cardContainer.style.display = 'grid';
    cardContainer.style.gridTemplateColumns = 'repeat(auto-fit, minmax(140px, 1fr))';
    cardContainer.style.gap = '10px';
    
    // Define stats with label, value, and icon
    const stats = [
        { label: 'Total Sessions', value: socStats.total_sessions, icon: '📊', color: '#3498db' },
        { label: 'Failed Sessions', value: socStats.failed_sessions, icon: '❌', color: '#e74c3c' },
        { label: 'End SoC > 80%', value: socStats.above_80_count, icon: '🔋', color: '#27ae60' },
        { label: 'End SoC = 80%', value: socStats.exactly_80_count, icon: '⚖️', color: '#f39c12' },
        { label: 'End SoC < 80%', value: socStats.below_80_count, icon: '⚠️', color: '#e67e22' },
        { label: 'End SoC = 100%', value: socStats.exactly_100_count, icon: '✅', color: '#2ecc71' }
    ];
    
    // Create more compact styled cards for each stat
    stats.forEach(stat => {
        const card = document.createElement('div');
        card.style.backgroundColor = '#ffffff';
        card.style.borderRadius = '6px';
        card.style.padding = '10px';
        card.style.boxShadow = '0 1px 3px rgba(0,0,0,0.1)';
        card.style.display = 'flex';
        card.style.flexDirection = 'column';
        card.style.transition = 'transform 0.2s';
        card.style.border = `1px solid ${stat.color}20`;
        
        // Add hover effect
        card.onmouseover = () => card.style.transform = 'translateY(-2px)';
        card.onmouseout = () => card.style.transform = 'translateY(0)';
        
        // Create header with icon and label
        const header = document.createElement('div');
        header.style.display = 'flex';
        header.style.alignItems = 'center';
        header.style.marginBottom = '5px';
        
        const icon = document.createElement('span');
        icon.textContent = stat.icon;
        icon.style.marginRight = '5px';
        icon.style.fontSize = '16px';
        
        const label = document.createElement('span');
        label.textContent = stat.label;
        label.style.fontWeight = 'bold';
        label.style.color = '#555555';
        label.style.fontSize = '12px';
        
        header.appendChild(icon);
        header.appendChild(label);
        
        // Create value display
        const value = document.createElement('div');
        value.textContent = stat.value;
        value.style.fontSize = '18px';
        value.style.fontWeight = '600';
        value.style.color = stat.color;
        value.style.marginTop = 'auto';
        
        // Add elements to the card
        card.appendChild(header);
        card.appendChild(value);
        cardContainer.appendChild(card);
    });
    
    // Add cards to stats section
    statsSection.appendChild(cardContainer);
    
    // Right side: SOC Summary with progress bars
    const socAveragesSection = document.createElement('div');
    socAveragesSection.style.display = 'flex';
    socAveragesSection.style.flexDirection = 'column';
    
    // Add section title
    const summaryTitle = document.createElement('h3');
    summaryTitle.textContent = 'State of Charge Summary';
    summaryTitle.style.margin = '0 0 10px 0';
    summaryTitle.style.color = '#333';
    socAveragesSection.appendChild(summaryTitle);
    
    // Ensure percentage values exist in socStats or use default values
    const averageStartSoc = socStats.average_start_soc !== undefined ? socStats.average_start_soc : 0;
    const averageEndSoc = socStats.average_end_soc !== undefined ? socStats.average_end_soc : 0;
    const lowestStartSoc = socStats.lowest_start_soc !== undefined ? socStats.lowest_start_soc : 0;
    const above80Percentage = socStats.above_80_percentage !== undefined ? socStats.above_80_percentage : 0;
    const above90Percentage = socStats.above_90_percentage !== undefined ? socStats.above_90_percentage : 0;
    
    // Add progress bars for average start and end SOC
    socAveragesSection.appendChild(
        createSOCProgressBar('Average Start SoC', averageStartSoc, '#e74c3c')
    );
    socAveragesSection.appendChild(
        createSOCProgressBar('Average End SoC', averageEndSoc, '#2ecc71')
    );
    socAveragesSection.appendChild(
        createSOCProgressBar('Lowest Start SoC', lowestStartSoc, '#f39c12')
    );
    
    // Add percentages with progress bars
    socAveragesSection.appendChild(
        createSOCProgressBar('Sessions ending with SoC > 80%', above80Percentage, '#3498db')
    );
    socAveragesSection.appendChild(
        createSOCProgressBar('Sessions ending with SoC > 90%', above90Percentage, '#9b59b6')
    );
    
    // Add both sections to the main container (side by side)
    mainContainer.appendChild(statsSection);
    mainContainer.appendChild(socAveragesSection);
    
    // Add the main container to the stats element
    socStatsEl.appendChild(mainContainer);
}

// Create overview scatterplot
function createOverviewScatterplot() {
    if (sessions.length === 0) return;

    const data = [{
        x: sessions.map(s => new Date(s.start_time)),
        y: sessions.map(s => s.energy_added_hvb),
        mode: 'markers',
        type: 'scatter',
        marker: {
            size: 10,
            color: 'blue'
        },
        text: sessions.map(s => `${new Date(s.start_time).toLocaleString()} - ${s.energy_added_hvb} kWh - ${s.location}`),
        hoverinfo: 'text'
    }];
    
    const layout = {
        title: 'Energy added per charging session',
        xaxis: { title: 'Date' },
        yaxis: { title: 'kWh' },
        showlegend: false,
        template: plotlyTemplate
    };
    
    Plotly.newPlot('overview-scatterplot', data, layout);
}

// Create average gridpower scatterplot
function createAverageGridpowerScatterplot() {
    if (sessions.length === 0) return;

    const data = sessions.map(s => ({
        x: Array.from({ length: s.grid_power_start.length }, (_, i) => i),
        y: s.grid_power_start,
        mode: 'lines',
        type: 'scatter',
        name: `Session ${new Date(s.start_time).toLocaleString()}`
    }));
    
    const layout = {
        title: 'Average Grid Power Across All Sessions',
        xaxis: { title: 'Session Time (minutes)' },
        yaxis: { title: 'Grid Power (kW)' },
        showlegend: false,
        template: plotlyTemplate
    };
    
    Plotly.newPlot('average-gridpower-scatterplot', data, layout);
}

// Create estimated battery capacity scatterplot
function createEstimatedBatteryCapacityScatterplot(capacityData) {
    if (!capacityData || capacityData.length === 0) return;

    // Separate monthly averages from raw data points
    const monthlyAverages = capacityData.filter(d => d.is_monthly_average === true);
    const rawDataPoints = capacityData.filter(d => d.is_monthly_average !== true);
    
    // If we have monthly averages, use them for the trend line
    // Otherwise, fall back to using all data points
    const trendPoints = monthlyAverages.length >= 2 ? monthlyAverages : capacityData;
    
    // Set up data arrays for all points
    const x = capacityData.map(d => new Date(d.date));
    const y = capacityData.map(d => d.estimated_battery_capacity);
    const rawY = capacityData.map(d => d.raw_capacity);
    const socChange = capacityData.map(d => d.soc_change);
    const trendY = capacityData.map(d => d.trend); // Trend line values
    
    // Get arrays for monthly trend points
    const trendX = trendPoints.map(d => new Date(d.date)).sort((a, b) => a - b);
    const trendValues = trendPoints.map((d, i) => {
        return {
            x: new Date(d.date),
            y: d.trend
        };
    }).sort((a, b) => a.x - b.x);
    
    // Get first and last points of the trend line for display
    const firstPoint = trendValues[0];
    const lastPoint = trendValues[trendValues.length - 1];
    
    const data = [
        {
            x: x,
            y: rawY,
            mode: 'markers',
            type: 'scatter',
            name: 'Raw Measurements',
            marker: {
                size: socChange.map(s => Math.max(5, Math.min(15, s/3))), // Size based on SoC change
                opacity: 0.6,
                color: 'lightgray'
            },
            hovertemplate: '<b>Date:</b> %{x|%Y-%m-%d}<br><b>Capacity:</b> %{y:.1f} kWh<br><b>SoC Change:</b> %{text:.1f}%',
            text: socChange
        },
        {
            x: trendX,
            y: trendValues.map(p => p.y),
            mode: 'markers',
            type: 'scatter',
            name: 'Monthly Averages',
            marker: {
                color: 'blue',
                size: 10,
                symbol: 'circle'
            },
            hoverinfo: 'x+y+text',
            hovertemplate: '<b>Month:</b> %{x|%Y-%m}<br><b>Avg Capacity:</b> %{y:.1f} kWh',
        },
        {
            x: [firstPoint.x, lastPoint.x],
            y: [firstPoint.y, lastPoint.y],
            mode: 'lines',
            type: 'scatter',
            name: 'Linear Trend',
            line: {
                color: 'red',
                width: 3
            }
        },
        {
            x: [firstPoint.x, lastPoint.x],
            y: [firstPoint.y, lastPoint.y],
            mode: 'text',
            text: [
                `Start: ${firstPoint.y.toFixed(1)} kWh`,
                `End: ${lastPoint.y.toFixed(1)} kWh<br>Loss: ${(firstPoint.y - lastPoint.y).toFixed(1)} kWh (${((firstPoint.y - lastPoint.y) / firstPoint.y * 100).toFixed(1)}%)`
            ],
            textposition: 'top center',
            showlegend: false
        }
    ];
    
    // Calculate a reasonable Y-axis range
    const validYValues = trendValues.map(p => p.y).filter(v => !isNaN(v) && isFinite(v));
    const minY = validYValues.length ? Math.min(...validYValues) * 0.95 : 0;
    const maxY = validYValues.length ? Math.max(...validYValues) * 1.05 : 100;
    
    const layout = {
        title: 'Estimated Battery Capacity (SoH) Over Time',
        xaxis: { 
            title: 'Date',
            type: 'date'
        },
        yaxis: { 
            title: 'Battery Capacity (kWh)',
            range: [minY, maxY]
        },
        legend: {
            orientation: 'h',
            y: -0.2
        },
        annotations: [
            {
                x: 0.5,
                y: 1.05,
                xref: 'paper',
                yref: 'paper',
                text: 'Monthly aggregated data with linear trend',
                showarrow: false,
                font: {
                    size: 12
                }
            }
        ],
        template: plotlyTemplate,
        hovermode: 'closest'
    };
    
    Plotly.newPlot('estimated-battery-capacity-scatterplot', data, layout);
}

// Initialize charging locations map
async function initChargingLocationsMap() {
    const mapContainerId = 'charging-locations-map';
    const mapContainer = document.getElementById(mapContainerId);
    if (!mapContainer) return;
    
    // Completely remove and recreate the map container element to ensure a clean state
    if (chargingLocationsMap) {
        try {
            // Remove the map and clear all handlers
            chargingLocationsMap.remove();
        } catch (e) {
            console.warn("Error removing map:", e);
        }
        chargingLocationsMap = null;
    }
    
    // Replace the map container with a new one
    const parentElement = mapContainer.parentElement;
    const oldContainer = document.getElementById(mapContainerId);
    
    if (parentElement && oldContainer) {
        // Remove the old container
        parentElement.removeChild(oldContainer);
        
        // Create a new container with the same ID
        const newContainer = document.createElement('div');
        newContainer.id = mapContainerId;
        newContainer.className = mapContainer.className;
        newContainer.style.cssText = 'width: 100%; height: 500px;'; // Set appropriate size
        
        // Add the new container to the parent
        parentElement.appendChild(newContainer);
    }
    
    // Prepare date parameters (consistent with other API calls)
    const startDate = startDateEl.value;
    const endDate = endDateEl.value;
    let dateParams = '';
    
    if (startDate && endDate) {
        dateParams = `?startDate=${startDate}&endDate=${endDate}`;
    }
    
    // Fetch map data with date filter
    try {
        const mapUrl = '/api/map' + dateParams;
        const response = await fetch(mapUrl);
        if (!response.ok) {
            throw new Error('Failed to fetch map data');
        }
        
        const locations = await response.json();
        
        if (locations.length === 0) {
            // If no locations, just create an empty map centered on Europe
            chargingLocationsMap = L.map('charging-locations-map').setView([50.0, 10.0], 4);
            
            // Add OpenStreetMap tile layer
            L.tileLayer('https://tiles.ext.ffmuc.net/osm/{z}/{x}/{y}.png', {
                attribution: 'Map data © OpenStreetMap contributors, Tiles © FFMUC'
            }).addTo(chargingLocationsMap);
            
            return;
        }
        
        // Calculate map center
        const latSum = locations.reduce((sum, loc) => sum + loc.latitude, 0);
        const lonSum = locations.reduce((sum, loc) => sum + loc.longitude, 0);
        const mapCenter = [latSum / locations.length, lonSum / locations.length];
        
        // Create a new map instance
        chargingLocationsMap = L.map('charging-locations-map').setView(mapCenter, 5);
        
        // Add OpenStreetMap tile layer
        L.tileLayer('https://tiles.ext.ffmuc.net/osm/{z}/{x}/{y}.png', {
            attribution: 'Map data © OpenStreetMap contributors, Tiles © FFMUC'
        }).addTo(chargingLocationsMap);
        
        // Add a legend to the map
        const legend = L.control({position: 'bottomright'});
        legend.onAdd = function(map) {
            const div = L.DomUtil.create('div', 'info legend');
            div.style.backgroundColor = 'white';
            div.style.padding = '6px';
            div.style.border = '1px solid #ccc';
            div.style.borderRadius = '5px';
            
            // Legend title
            div.innerHTML = '<h4 style="margin:0 0 5px 0;">Number of Sessions</h4>';
            
            // Legend items
            const sessionCounts = [1, 2, 4, 6, 11];
            const labels = [];
            
            // Loop through our intervals and generate a colored square for each interval
            for (let i = 0; i < sessionCounts.length; i++) {
                const color = getMarkerHexColor(getMarkerColor(sessionCounts[i]));
                labels.push(
                    '<i style="background:' + color + '; width:15px; height:15px; display:inline-block;"></i> ' +
                    (i === sessionCounts.length - 1 ? '> ' + (sessionCounts[i]-1) + ' sessions' :
                    (i === 0 ? '1 session' : sessionCounts[i-1] + '&ndash;' + (sessionCounts[i]) + ' sessions'))
                );
            }
            
            // Add failed sessions to legend
            labels.push('<i style="background:' + getMarkerHexColor('blue') + '; width:15px; height:15px; display:inline-block;"></i> Location with failed sessions');
            
            div.innerHTML += labels.join('<br>');
            return div;
        };
        legend.addTo(chargingLocationsMap);
        
        // Add markers for each location
        locations.forEach(location => {
            // Skip if no valid coordinates
            if (!location.latitude || !location.longitude) return;
            
            // Choose marker color based on session count, but highlight failed sessions
            const markerColor = location.failed_count > 0 ? 'blue' : getMarkerColor(location.session_count);
            
            // Calculate marker size based on the number of sessions (min 8, max 16)
            const radius = Math.max(8, Math.min(16, 8 + location.session_count / 2));
            
            // Create custom marker options with colors
            const markerOptions = {
                radius: radius,
                fillColor: getMarkerHexColor(markerColor),
                color: "#000",
                weight: 1,
                opacity: 1,
                fillOpacity: 0.8
            };
            
            // Create a circle marker instead of an icon marker (more reliable)
            L.circleMarker([location.latitude, location.longitude], markerOptions).addTo(chargingLocationsMap)
            .bindPopup(`${location.name} (Provider: ${location.provider || 'Unknown'})
                       <br>Total Sessions: ${location.session_count}
                       <br>Successful Sessions: ${location.success_count}
                       <br>Failed Sessions: ${location.failed_count}
                       <br>Total Energy Added: ${location.total_energy.toFixed(2)} kWh`);
        });
        
    } catch (error) {
        console.error('Error loading map data:', error);
    }
}

// Get marker color based on session count
function getMarkerColor(count) {
    if (count > 10) return 'darkred';
    if (count > 5) return 'red';
    if (count > 3) return 'orange';
    if (count > 1) return 'yellow';
    return 'green';
}

// Convert color names to hex codes for circle markers
function getMarkerHexColor(colorName) {
    const colorMap = {
        'darkred': '#8B0000',
        'red': '#FF0000',
        'orange': '#FFA500',
        'yellow': '#FFFF00',
        'green': '#008000',
        'blue': '#0000FF',
        'lightred': '#FF7F7F'
    };
    
    return colorMap[colorName] || '#3388ff'; // Default to Leaflet blue
}

// Create charge details graph
function createChargeDetailsGraph() {
    if (!currentSession) return;
    
    const data = [{
        x: [new Date(currentSession.start_time), new Date(currentSession.end_time)],
        y: [currentSession.soc_start, currentSession.soc_end],
        mode: 'lines+markers',
        type: 'scatter',
        marker: { size: 10, color: 'blue' }
    }];
    
    const layout = {
        title: 'Charge Details',
        xaxis: { title: 'Time' },
        yaxis: { title: 'SOC (%)' },
        template: plotlyTemplate
    };
    
    Plotly.newPlot('charge-details-graph', data, layout);
}

// Create modern session stats cards
function createCombinedGauges() {
    if (!currentSession || !sessions.length) return;
    
    // Get the container element
    const container = document.getElementById('combined-gauges');
    container.innerHTML = '';
    
    // Create a modern card layout
    const cardContainer = document.createElement('div');
    cardContainer.style.display = 'grid';
    cardContainer.style.gridTemplateColumns = 'repeat(auto-fit, minmax(200px, 1fr))';
    cardContainer.style.gap = '20px';
    cardContainer.style.padding = '20px';
    cardContainer.style.height = '100%';
    cardContainer.style.backgroundColor = '#ffffff';
    cardContainer.style.borderRadius = '15px';
    cardContainer.style.boxShadow = '0 5px 15px rgba(0, 0, 0, 0.05)';
    
    // Find max values for comparison
    const maxPower = Math.max(...sessions.map(s => s.avg_power));
    const maxCost = Math.max(...sessions.map(s => s.cost));
    const maxEnergy = Math.max(...sessions.map(s => s.energy_added_hvb));
    const maxTime = Math.max(...sessions.map(s => s.session_time_minutes));
    
    // Define session metrics
    const metrics = [
        {
            label: "Average Grid Power",
            value: currentSession.avg_power.toFixed(1),
            unit: "kW",
            icon: "⚡",
            color: "#3498db",
            max: maxPower,
            percentOfMax: (currentSession.avg_power / maxPower) * 100
        },
        {
            label: "Cost",
            value: currentSession.cost.toFixed(2),
            unit: "€",
            icon: "💶",
            color: "#2ecc71",
            max: maxCost,
            percentOfMax: (currentSession.cost / maxCost) * 100
        },
        {
            label: "Efficiency",
            value: (currentSession.efficiency * 100).toFixed(1),
            unit: "%",
            icon: "🔋",
            color: "#f39c12",
            max: 100,
            percentOfMax: currentSession.efficiency * 100
        },
        {
            label: "Energy Added",
            value: currentSession.energy_added_hvb.toFixed(1),
            unit: "kWh",
            icon: "⚡",
            color: "#9b59b6",
            max: maxEnergy,
            percentOfMax: (currentSession.energy_added_hvb / maxEnergy) * 100
        },
        {
            label: "Session Time",
            value: Math.round(currentSession.session_time_minutes),
            unit: "min",
            icon: "⏱️",
            color: "#e74c3c",
            max: maxTime,
            percentOfMax: (currentSession.session_time_minutes / maxTime) * 100
        }
    ];
    
    // Create a card for each metric
    metrics.forEach(metric => {
        const card = createMetricCard(metric);
        cardContainer.appendChild(card);
    });
    
    // Add additional session info card
    const additionalInfo = createAdditionalInfoCard();
    cardContainer.appendChild(additionalInfo);
    
    // Add to the container
    container.appendChild(cardContainer);
    
    // Helper function to create a metric card
    function createMetricCard(metric) {
        const card = document.createElement('div');
        card.className = 'session-metric-card';
        card.style.backgroundColor = '#ffffff';
        card.style.borderRadius = '12px';
        card.style.padding = '20px';
        card.style.boxShadow = '0 4px 12px rgba(0,0,0,0.08)';
        card.style.display = 'flex';
        card.style.flexDirection = 'column';
        card.style.alignItems = 'center';
        card.style.justifyContent = 'space-between';
        card.style.transition = 'transform 0.3s ease, box-shadow 0.3s ease';
        card.style.position = 'relative';
        card.style.overflow = 'hidden';
        
        // Add hover effect
        card.onmouseover = () => {
            card.style.transform = 'translateY(-5px)';
            card.style.boxShadow = '0 8px 20px rgba(0,0,0,0.12)';
        };
        card.onmouseout = () => {
            card.style.transform = 'translateY(0)';
            card.style.boxShadow = '0 4px 12px rgba(0,0,0,0.08)';
        };
        
        // Create icon
        const icon = document.createElement('div');
        icon.textContent = metric.icon;
        icon.style.fontSize = '28px';
        icon.style.marginBottom = '10px';
        icon.style.backgroundColor = `${metric.color}15`; // Light background based on color
        icon.style.width = '50px';
        icon.style.height = '50px';
        icon.style.borderRadius = '50%';
        icon.style.display = 'flex';
        icon.style.justifyContent = 'center';
        icon.style.alignItems = 'center';
        
        // Create label
        const label = document.createElement('div');
        label.textContent = metric.label;
        label.style.fontSize = '14px';
        label.style.color = '#666';
        label.style.marginBottom = '15px';
        label.style.textAlign = 'center';
        
        // Create value container
        const valueContainer = document.createElement('div');
        valueContainer.style.display = 'flex';
        valueContainer.style.alignItems = 'baseline';
        valueContainer.style.marginBottom = '15px';
        
        // Create value
        const value = document.createElement('div');
        value.textContent = metric.value;
        value.style.fontSize = '32px';
        value.style.fontWeight = 'bold';
        value.style.color = metric.color;
        
        // Create unit
        const unit = document.createElement('div');
        unit.textContent = metric.unit;
        unit.style.fontSize = '14px';
        unit.style.color = '#999';
        unit.style.marginLeft = '5px';
        
        valueContainer.appendChild(value);
        valueContainer.appendChild(unit);
        
        // Create progress bar container
        const progressContainer = document.createElement('div');
        progressContainer.style.width = '100%';
        progressContainer.style.height = '6px';
        progressContainer.style.backgroundColor = '#f0f0f0';
        progressContainer.style.borderRadius = '3px';
        progressContainer.style.overflow = 'hidden';
        progressContainer.style.marginTop = '5px';
        
        // Create progress bar
        const progress = document.createElement('div');
        progress.style.width = `${metric.percentOfMax}%`;
        progress.style.height = '100%';
        progress.style.backgroundColor = metric.color;
        progress.style.borderRadius = '3px';
        progress.style.transition = 'width 1s ease';
        
        progressContainer.appendChild(progress);
        
        // Create comparison text
        const comparison = document.createElement('div');
        comparison.textContent = `${metric.percentOfMax.toFixed(0)}% of max (${metric.max.toFixed(1)} ${metric.unit})`;
        comparison.style.fontSize = '10px';
        comparison.style.color = '#999';
        comparison.style.marginTop = '5px';
        comparison.style.textAlign = 'right';
        
        // Assemble the card
        card.appendChild(icon);
        card.appendChild(label);
        card.appendChild(valueContainer);
        card.appendChild(progressContainer);
        card.appendChild(comparison);
        
        return card;
    }
    
    // Helper function to create additional info card
    function createAdditionalInfoCard() {
        const card = document.createElement('div');
        card.className = 'session-info-card';
        card.style.backgroundColor = '#ffffff';
        card.style.borderRadius = '12px';
        card.style.padding = '20px';
        card.style.boxShadow = '0 4px 12px rgba(0,0,0,0.08)';
        card.style.display = 'flex';
        card.style.flexDirection = 'column';
        card.style.transition = 'transform 0.3s ease, box-shadow 0.3s ease';
        
        // Add hover effect
        card.onmouseover = () => {
            card.style.transform = 'translateY(-5px)';
            card.style.boxShadow = '0 8px 20px rgba(0,0,0,0.12)';
        };
        card.onmouseout = () => {
            card.style.transform = 'translateY(0)';
            card.style.boxShadow = '0 4px 12px rgba(0,0,0,0.08)';
        };
        
        // Create header
        const header = document.createElement('div');
        header.style.display = 'flex';
        header.style.alignItems = 'center';
        header.style.marginBottom = '15px';
        
        const icon = document.createElement('span');
        icon.textContent = '📊';
        icon.style.fontSize = '24px';
        icon.style.marginRight = '10px';
        
        const title = document.createElement('h4');
        title.textContent = 'Session Details';
        title.style.margin = '0';
        title.style.fontSize = '16px';
        title.style.color = '#444';
        
        header.appendChild(icon);
        header.appendChild(title);
        
        // Create content
        const content = document.createElement('div');
        
        // Format date and time
        const startDate = new Date(currentSession.start_time);
        const endDate = new Date(currentSession.end_time);
        const dateOptions = { weekday: 'short', year: 'numeric', month: 'short', day: 'numeric' };
        const timeOptions = { hour: '2-digit', minute: '2-digit' };
        
        const formattedDate = startDate.toLocaleDateString(undefined, dateOptions);
        const formattedStartTime = startDate.toLocaleTimeString(undefined, timeOptions);
        const formattedEndTime = endDate.toLocaleTimeString(undefined, timeOptions);
        
        // Create info items
        const infoItems = [
            { label: "Date", value: formattedDate },
            { label: "Time", value: `${formattedStartTime} - ${formattedEndTime}` },
            { label: "Location", value: currentSession.location || "Unknown" },
            { label: "SoC Change", value: `${currentSession.soc_start}% → ${currentSession.soc_end}%` }
        ];
        
        // Create a table for info items
        const table = document.createElement('table');
        table.style.width = '100%';
        table.style.borderCollapse = 'collapse';
        
        infoItems.forEach(item => {
            const row = document.createElement('tr');
            
            const labelCell = document.createElement('td');
            labelCell.textContent = item.label;
            labelCell.style.padding = '5px 0';
            labelCell.style.color = '#666';
            labelCell.style.fontSize = '14px';
            
            const valueCell = document.createElement('td');
            valueCell.textContent = item.value;
            valueCell.style.padding = '5px 0';
            valueCell.style.textAlign = 'right';
            valueCell.style.fontWeight = '500';
            valueCell.style.color = '#333';
            valueCell.style.fontSize = '14px';
            
            row.appendChild(labelCell);
            row.appendChild(valueCell);
            table.appendChild(row);
        });
        
        content.appendChild(table);
        
        // Add SOC progress bar
        const socProgressContainer = document.createElement('div');
        socProgressContainer.style.marginTop = '15px';
        
        const socLabel = document.createElement('div');
        socLabel.textContent = 'Charge Progress';
        socLabel.style.fontSize = '14px';
        socLabel.style.color = '#666';
        socLabel.style.marginBottom = '5px';
        
        const socBarContainer = document.createElement('div');
        socBarContainer.style.height = '20px';
        socBarContainer.style.backgroundColor = '#f0f0f0';
        socBarContainer.style.borderRadius = '10px';
        socBarContainer.style.position = 'relative';
        socBarContainer.style.overflow = 'hidden';
        
        const startBar = document.createElement('div');
        startBar.style.position = 'absolute';
        startBar.style.left = '0';
        startBar.style.top = '0';
        startBar.style.width = `${currentSession.soc_start}%`;
        startBar.style.height = '100%';
        startBar.style.backgroundColor = '#FFB74D'; // Orange for start
        
        // Create a difference bar to show the charging progress with a different color
        const diffBar = document.createElement('div');
        diffBar.style.position = 'absolute';
        diffBar.style.left = `${currentSession.soc_start}%`;
        diffBar.style.top = '0';
        diffBar.style.width = `${currentSession.soc_end - currentSession.soc_start}%`;
        diffBar.style.height = '100%';
        diffBar.style.backgroundColor = '#3498db'; // Blue for the difference
        diffBar.style.transition = 'width 1s ease-in-out';
        
        // Keep the end bar for reference but make it transparent
        const endBar = document.createElement('div');
        endBar.style.position = 'absolute';
        endBar.style.left = '0';
        endBar.style.top = '0';
        endBar.style.width = '0'; // No width needed as we're showing the difference separately
        endBar.style.height = '100%';
        endBar.style.backgroundColor = 'transparent'; // Make it transparent
        
        // Add values on the progress bar
        const startValue = document.createElement('span');
        startValue.textContent = `${currentSession.soc_start}%`;
        startValue.style.position = 'absolute';
        startValue.style.left = `${Math.min(Math.max(currentSession.soc_start - 10, 0), 90)}%`;
        startValue.style.top = '0';
        startValue.style.fontSize = '12px';
        startValue.style.fontWeight = 'bold';
        startValue.style.color = '#fff';
        startValue.style.padding = '2px 5px';
        
        const endValue = document.createElement('span');
        endValue.textContent = `${currentSession.soc_end}%`;
        endValue.style.position = 'absolute';
        endValue.style.right = `${Math.min(Math.max(100 - currentSession.soc_end - 10, 0), 90)}%`;
        endValue.style.top = '0';
        endValue.style.fontSize = '12px';
        endValue.style.fontWeight = 'bold';
        endValue.style.color = '#fff';
        endValue.style.padding = '2px 5px';
        
        // Assemble SOC progress bar
        socBarContainer.appendChild(startBar);
        socBarContainer.appendChild(diffBar); // Add the difference bar
        socBarContainer.appendChild(endBar);
        socBarContainer.appendChild(startValue);
        socBarContainer.appendChild(endValue);
        
        socProgressContainer.appendChild(socLabel);
        socProgressContainer.appendChild(socBarContainer);
        
        // Assemble the card
        card.appendChild(header);
        card.appendChild(content);
        card.appendChild(socProgressContainer);
        
        return card;
    }
}

// Create grid power graph
function createGridPowerGraph() {
    if (!currentSession || !currentSession.grid_power_start.length) return;
    
    // Calculate time points for x-axis
    const startTime = new Date(currentSession.start_time).getTime();
    const endTime = new Date(currentSession.end_time).getTime();
    const timeRange = endTime - startTime;
    const timePoints = currentSession.grid_power_start.map((_, i) => 
        new Date(startTime + (i * timeRange / currentSession.grid_power_start.length))
    );
    
    // Basic grid power data
    const data = [{
        x: timePoints,
        y: currentSession.grid_power_start,
        mode: 'lines+markers',
        type: 'scatter',
        marker: { size: 6, color: 'green' },
        line: { color: 'green' }
    }];
    
    // Add peak marker if there are values
    if (currentSession.grid_power_start.length > 0) {
        const peakValue = Math.max(...currentSession.grid_power_start);
        const peakIndex = currentSession.grid_power_start.indexOf(peakValue);
        const peakTime = timePoints[peakIndex];
        
        data.push({
            x: [peakTime],
            y: [peakValue],
            mode: 'markers+text',
            type: 'scatter',
            marker: { size: 12, color: 'red', symbol: 'x' },
            text: [`Peak: ${peakValue.toFixed(2)} kW`],
            textposition: 'bottom center',
            name: 'Peak'
        });
    }
    
    const layout = {
        title: 'Grid Power Over Time',
        xaxis: { title: 'Time' },
        yaxis: { title: 'Grid Power (kW)' },
        template: plotlyTemplate
    };
    
    Plotly.newPlot('grid-power-graph', data, layout);
}

// Create range map
function createRangeMap() {
    if (!currentSession) return;
    
    const mapContainerId = 'range-map';
    const mapContainer = document.getElementById(mapContainerId);
    if (!mapContainer) return;
    
    console.log("Creating range map - container found:", mapContainer);
    
    // Replace the map container with a new one
    const parentElement = mapContainer.parentElement;
    const oldContainer = document.getElementById(mapContainerId);
    
    if (parentElement && oldContainer) {
        // Remove the old container
        parentElement.removeChild(oldContainer);
        
        // Create a new container with the same ID
        const newContainer = document.createElement('div');
        newContainer.id = mapContainerId;
        newContainer.className = mapContainer.className;
        newContainer.style.cssText = 'width: 100%; height: 400px;'; // Set appropriate size
        
        // Add the new container to the parent
        parentElement.appendChild(newContainer);
        console.log("Map container recreated");
    }
    
    // Create map focused on current session location
    const rangeMap = L.map('range-map').setView([currentSession.latitude, currentSession.longitude], 13);
    
    // Add OpenStreetMap tile layer
    L.tileLayer('https://tiles.ext.ffmuc.net/osm/{z}/{x}/{y}.png', {
        attribution: 'Map data © OpenStreetMap contributors, Tiles © FFMUC'
    }).addTo(rangeMap);
    
    // Add marker for the current session using a circle marker
    // For individual sessions, we'll use a fixed color scheme:
    // - Blue for failed sessions (SoC didn't change)
    // - Green for successful sessions
    const markerOptions = {
        radius: 10,
        fillColor: currentSession.soc_end === currentSession.soc_start ? '#0000FF' : '#008000', // Blue for failed, Green for successful
        color: "#000",
        weight: 1,
        opacity: 1,
        fillOpacity: 0.8
    };
    
    const sessionStatus = currentSession.soc_end === currentSession.soc_start ? 'Failed Session' : 'Successful Session';
    
    L.circleMarker([currentSession.latitude, currentSession.longitude], markerOptions)
        .addTo(rangeMap)
        .bindPopup(`${currentSession.location}<br>${sessionStatus}<br>Energy Added: ${currentSession.energy_added_hvb.toFixed(2)} kWh`);
}

// Initialize the application
document.addEventListener('DOMContentLoaded', init);

// Expose initialization function for dynamic script loading
function initBMWTools() {
    // If DOM is already loaded, initialize immediately
    if (document.readyState === 'complete' || document.readyState === 'interactive') {
        init();
    } else {
        // Otherwise wait for DOMContentLoaded
        document.addEventListener('DOMContentLoaded', init);
    }
}
