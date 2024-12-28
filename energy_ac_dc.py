import pandas as pd
import json
import plotly.graph_objects as go

# Load the JSON data
with open('./path_to_your_json_file.json') as f:
    data = json.load(f)

# Initialize variables to store AC and DC energy and session counts
ac_energy = 0
dc_energy = 0
dc_onec_sessions = 0
dc_onec_energy = 0
ac_sessions = 0
dc_sessions = 0

# Loop through each session and its charging blocks
for session in data:
    if 'chargingBlocks' in session and session['chargingBlocks']:
        # Check if there are any charging blocks to avoid empty max() issue
        session_energy = session.get('energyConsumedFromPowerGridKwh', 0)  # Total energy for session
        block_powers = [block['averagePowerGridKw'] for block in session['chargingBlocks']]
        
        if block_powers:  # Proceed only if the block_powers list is not empty
            max_power = max(block_powers)  # Max power in session
            
            # Classify charging type and accumulate energy and session count
            if max_power <= 12:  # AC Charging
                ac_energy += session_energy
                ac_sessions += 1
            elif max_power > 12 and max_power <= 80 :  # DC Charging below one C
                dc_onec_energy += session_energy
                dc_onec_sessions += 1
            else:  # DC Charging
                dc_energy += session_energy
                dc_sessions += 1

# Check the results
print(f"Total AC Energy: {ac_energy} kWh, Total AC Sessions: {ac_sessions}")
print(f"Total DC Energy: {dc_energy + dc_onec_energy} kWh, Total DC Sessions: {dc_sessions + dc_onec_sessions}")
print(f"Total DC OneC Sessions: {dc_onec_sessions}")

# Create a DataFrame to hold the results
df = pd.DataFrame({
    'Charging Type': ['AC Charging', 'DC Charging'],
    'Total Energy (kWh)': [ac_energy, dc_energy+dc_onec_energy],
    'Total Sessions': [ac_sessions, dc_sessions+dc_onec_sessions],
    'DC OneC Sessions': [0, dc_onec_sessions]  # Only DC Charging has OneC Sessions
})

# Plot the energy data using Plotly
fig = go.Figure()

# Add AC Charging bar
fig.add_trace(go.Bar(
    name='AC Charging',
    x=['AC Charging'],
    y=[ac_energy],
    marker_color='blue'
))

# Add DC OneC Sessions bar
fig.add_trace(go.Bar(
    name='DC Below One C Sessions',
    x=['DC Charging'],
    y=[dc_onec_energy],
    marker_color='green'
))

# Add remaining DC Sessions bar
fig.add_trace(go.Bar(
    name='DC Charging',
    x=['DC Charging'],
    y=[dc_energy],
    marker_color='orange'
))

# Format the y-axis to display numbers with 'kWh' and limit decimal places
fig.update_layout(
    title='Total Energy Charged - AC vs DC',
    xaxis_title='Charging Type',
    yaxis_title='Total Energy (kWh)',
    yaxis=dict(
        tickformat=".2f",  # Display with 2 decimal places
        ticksuffix=" kWh"  # Add kWh suffix
    ),
    barmode='stack',  # Stack the bars
    showlegend=True
)

# Add annotations for total number of sessions
for i, row in df.iterrows():
    if row['Charging Type'] == 'DC Charging':
        fig.add_annotation(
            x=row['Charging Type'],
            y=row['Total Energy (kWh)'],
            text=f"Total Sessions: {row['Total Sessions']} / DC Below One C Sessions: {row['DC OneC Sessions']}",
            showarrow=False,
            yshift=10  # Position the annotation slightly above the bar
        )
    else:
        fig.add_annotation(
            x=row['Charging Type'],
            y=row['Total Energy (kWh)'],
            text=f"Total Sessions: {row['Total Sessions']}",
            showarrow=False,
            yshift=10  # Position the annotation slightly above the bar
        )

# Show the figure
fig.show()