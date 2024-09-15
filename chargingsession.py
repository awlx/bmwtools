import json
import pandas as pd
import plotly.graph_objs as go
import datetime

# Load the JSON data
with open('./path_to_your_json_file.json', 'r') as file:
    data = json.load(file)

# Filter out sessions where the peak averagePowerGridKw is below 20kW
filtered_data = [
    session for session in data if session.get("chargingBlocks") and max(block['averagePowerGridKw'] for block in session["chargingBlocks"]) >= 20
]

# Extract relevant information
session_data = [{
    'sessionIndex': i,
    'chargingDurationSec': session['endTime'] - session['startTime'],
    'energyIncreaseHvbKwh': session['energyIncreaseHvbKwh'],
    'sessionTime': datetime.datetime.utcfromtimestamp(session['endTime'])
} for i, session in enumerate(filtered_data)]

# Convert to DataFrame
df = pd.DataFrame(session_data)

# Filter out sessions with zero energy increase and time over 200 minutes
filtered_df = df[(df['chargingDurationSec'] > 60) & (df['chargingDurationSec'] <= 200 * 60) & (df['energyIncreaseHvbKwh'] > 0)]

# Convert time to minutes
filtered_df['chargingDurationMin'] = filtered_df['chargingDurationSec'] / 60.0

# Split the data based on the time condition
date_threshold = datetime.datetime(2024, 9, 1)
before_September_df = filtered_df[filtered_df['sessionTime'] < date_threshold]
after_September_df = filtered_df[filtered_df['sessionTime'] >= date_threshold]

# Create the plot
fig12 = go.Figure()

# Plot sessions before September 2024 in blue
fig12.add_trace(go.Scatter(
    x=before_September_df['chargingDurationMin'],
    y=before_September_df['energyIncreaseHvbKwh'],
    mode='markers',
    marker=dict(size=10, color='blue'),
    name='Before September 2024'
))

# Plot sessions from September 2024 onwards in red
fig12.add_trace(go.Scatter(
    x=after_September_df['chargingDurationMin'],
    y=after_September_df['energyIncreaseHvbKwh'],
    mode='markers',
    marker=dict(size=10, color='red'),
    name='September 2024 onwards'
))

fig12.update_layout(
    title='Energy Increase (kWh) vs Charging Session Duration (min)',
    xaxis_title='Charging Session Duration (min)',
    yaxis_title='Energy Increase (kWh)',
    template='plotly_dark'
)

# Save the plot as an HTML file
fig12.write_html("./filtered_peak_energy_increase_vs_charging_session_fixed.html")
