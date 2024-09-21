import json
import pandas as pd
import plotly.express as px

# Load the JSON file
with open('path_to_your_json_file.json', 'r') as file:
    data = json.load(file)

# Prepare lists to store results
session_dates = []
energy_wasted_percentage = []
charging_type = []  # To store whether it's AC or DC charging

# Loop through each charging session
for entry in data:
    # Extract relevant values for energy consumed from grid and energy delivered to the battery
    energy_from_grid = entry.get('energyConsumedFromPowerGridKwh', 0)
    energy_to_battery = entry.get('energyIncreaseHvbKwh', 0)
    
    # Filter out sessions with an energy increase to the battery below 5kWh
    if energy_to_battery < 5:
        continue
    
    # Determine if the session is AC or DC by checking if any charging block has an average power > 11kW
    is_dc = any(block['averagePowerGridKw'] > 11 for block in entry['chargingBlocks'])
    session_type = 'DC' if is_dc else 'AC'
    
    # Calculate energy wasted in percentage
    if energy_from_grid > 0:
        energy_wasted = ((energy_from_grid - energy_to_battery) / energy_from_grid) * 100
        if energy_wasted >= 0:  # Only keep sessions where wasted energy is non-negative
            session_start_time = pd.to_datetime(entry['startTime'], unit='s')
            session_dates.append(session_start_time)
            energy_wasted_percentage.append(energy_wasted)
            charging_type.append(session_type)

# Create a DataFrame using the extracted data
df = pd.DataFrame({
    'Date': session_dates,
    'Energy Wasted (%)': energy_wasted_percentage,
    'Charging Type': charging_type
})

# Sort the data by date
df = df.sort_values(by='Date')

# Create a scatter plot using Plotly
fig = px.scatter(df, x='Date', y='Energy Wasted (%)', color='Charging Type', 
                 title='Energy Wasted (%) per Charging Session (AC vs DC)',
                 labels={'Date': 'Date', 'Energy Wasted (%)': 'Energy Wasted (%)'},
                 color_discrete_map={'AC': 'blue', 'DC': 'red'},
                 hover_data=['Charging Type'])

# Customize the layout
fig.update_layout(xaxis_title='Date', yaxis_title='Energy Wasted (%)')

# Show the plot
fig.show()
