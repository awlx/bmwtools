import pandas as pd
import json
import plotly.graph_objects as go

# Load the JSON data
with open('./path_to_your_json_file.json') as f:
    data = json.load(f)

# Initialize variables to store AC and DC energy and session counts
ac_energy = 0
dc_energy = 0
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
            if max_power <= 11:  # AC Charging
                ac_energy += session_energy
                ac_sessions += 1
            else:  # DC Charging
                dc_energy += session_energy
                dc_sessions += 1

# Check the results
print(f"Total AC Energy: {ac_energy} kWh, Total AC Sessions: {ac_sessions}")
print(f"Total DC Energy: {dc_energy} kWh, Total DC Sessions: {dc_sessions}")

# Create a DataFrame to hold the results
df = pd.DataFrame({
    'Charging Type': ['AC Charging', 'DC Charging'],
    'Total Energy (kWh)': [ac_energy, dc_energy],
    'Total Sessions': [ac_sessions, dc_sessions]
})

# Plot the energy data using Plotly
fig = go.Figure(data=[
    go.Bar(name='Charging Type', x=df['Charging Type'], y=df['Total Energy (kWh)'])
])

# Format the y-axis to display numbers with 'kWh' and limit decimal places
fig.update_layout(
    title='Total Energy Charged - AC vs DC',
    xaxis_title='Charging Type',
    yaxis_title='Total Energy (kWh)',
    yaxis=dict(
        tickformat=".2f",  # Display with 2 decimal places
        ticksuffix=" kWh"  # Add kWh suffix
    ),
    showlegend=False
)

# Add annotations for total number of sessions
for i, row in df.iterrows():
    fig.add_annotation(
        x=row['Charging Type'],
        y=row['Total Energy (kWh)'],
        text=f"Sessions: {row['Total Sessions']}",
        showarrow=False,
        yshift=10  # Position the annotation slightly above the bar
    )

# Show the figure
fig.show()
