import pandas as pd
import json
import plotly.graph_objects as go
from plotly.subplots import make_subplots

# Load JSON data
with open('path_to_your_json_file.json') as file:
    data = json.load(file)

# Initialize lists to hold data
dates = []
wasted_energy = []
charged_energy = []
session_types = []  # List to identify AC or DC
preconditioning = []  # List to store preconditioning status
power_grid = []  # Total power taken from the grid
power_hvb = []  # Total power for High Voltage battery

for session in data:
    if 'energyConsumedFromPowerGridKwh' in session and 'energyIncreaseHvbKwh' in session:
        grid_energy = session['energyConsumedFromPowerGridKwh']
        hvb_energy = session['energyIncreaseHvbKwh']
        energy_waste = abs(grid_energy - hvb_energy)
        wasted_energy.append(energy_waste)
        charged_energy.append(hvb_energy)
        power_grid.append(grid_energy)
        power_hvb.append(hvb_energy)
        dates.append(pd.to_datetime(session['startTime'], unit='s'))  # Convert timestamp to datetime

        total_power = 0
        count = 0
        if 'chargingBlocks' in session:
            for block in session['chargingBlocks']:
                if 'averagePowerGridKw' in block:
                    total_power += block['averagePowerGridKw']
                    count += 1

        average_power = total_power / count if count > 0 else 0
        is_ac = average_power <= 11
        session_types.append('AC' if is_ac else 'DC')
        preconditioning.append(session.get('isPreconditioningActivated', False))

# Create a DataFrame
df = pd.DataFrame({
    'Date': dates,
    'Wasted Energy (kWh)': wasted_energy,
    'Charged Energy (kWh)': charged_energy,
    'Session Type': session_types,
    'Preconditioning': preconditioning,
    'Power Grid (kWh)': power_grid,
    'Power HVB (kWh)': power_hvb
}).sort_values('Date')

# Summarize total wasted and charged energy
total_wasted_energy = df['Wasted Energy (kWh)'].sum()
total_charged_energy = df['Charged Energy (kWh)'].sum()

# Create subplots
fig = make_subplots(specs=[[{"secondary_y": True}]])
colors = {'AC': 'blue', 'DC': 'red'}

# Plot wasted energy
for session_type, group in df.groupby('Session Type'):
    fig.add_trace(go.Scatter(x=group['Date'], y=group['Wasted Energy (kWh)'], mode='lines+markers',
                             name=f'{session_type} Wasted Energy', marker=dict(color=colors[session_type])),
                  secondary_y=False)

    # Plot markers for preconditioning status
    group_pre = group[group['Preconditioning']]
    fig.add_trace(go.Scatter(x=group_pre['Date'], y=group_pre['Wasted Energy (kWh)'], mode='markers', marker_symbol='triangle-up',
                             name=f'{session_type} Preconditioning Active', marker=dict(color=colors[session_type], size=10)),
                  secondary_y=False)

# Plot charged energy
for session_type, group in df.groupby('Session Type'):
    fig.add_trace(go.Scatter(x=group['Date'], y=group['Charged Energy (kWh)'], mode='lines',
                             name=f'{session_type} Charged Energy', line=dict(dash='dot', color=colors[session_type])),
                  secondary_y=True)

# Set axis titles
fig.update_layout(title='Energy Wasted and Charged During Charging Sessions',
                  xaxis_title='Date',
                  yaxis_title='Wasted Energy (kWh)',
                  annotations=[
                      dict(
                          x=0.5,
                          y=-0.2,
                          showarrow=False,
                          text=f'Total Wasted Energy: {total_wasted_energy:.2f} kWh, Total Charged Energy: {total_charged_energy:.2f} kWh',
                          xref="paper",
                          yref="paper"
                      )
                  ])
fig.update_yaxes(title_text='Charged Energy (kWh)', secondary_y=True)

# Show the figure
fig.show()
