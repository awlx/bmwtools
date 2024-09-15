import pandas as pd
import matplotlib.pyplot as plt
import json

# Load JSON data
with open('path_to_your_json_file.json') as file:
    data = json.load(file)

# Initialize lists to hold data
dates = []
ac_wasted_energy = []
ac_charged_energy = []
dc_wasted_energy = []
dc_charged_energy = []

for session in data:
    if 'energyConsumedFromPowerGridKwh' in session and 'energyIncreaseHvbKwh' in session:
        grid_energy = session['energyConsumedFromPowerGridKwh']
        hvb_energy = session['energyIncreaseHvbKwh']
        energy_waste = abs(grid_energy - hvb_energy)
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

        if is_ac:
            ac_wasted_energy.append(energy_waste)
            ac_charged_energy.append(hvb_energy)
            dc_wasted_energy.append(0)  # DC sessions have no wasted energy
            dc_charged_energy.append(0)
        else:
            ac_wasted_energy.append(0)
            ac_charged_energy.append(0)
            dc_wasted_energy.append(energy_waste)
            dc_charged_energy.append(hvb_energy)

# Create a DataFrame
df = pd.DataFrame({
    'Date': dates,
    'AC Wasted Energy (kWh)': ac_wasted_energy,
    'AC Charged Energy (kWh)': ac_charged_energy,
    'DC Wasted Energy (kWh)': dc_wasted_energy,
    'DC Charged Energy (kWh)': dc_charged_energy,
})

# Plotting
fig, ax = plt.subplots(figsize=(12, 7))

# Plot bars for AC wasted energy and charged energy
ax.bar(df['Date'], df['AC Wasted Energy (kWh)'], color='red', label='AC Wasted Energy')
ax.bar(df['Date'], df['AC Charged Energy (kWh)'], bottom=df['AC Wasted Energy (kWh)'], color='lime', label='AC Charged Energy')

# Plot bars for DC wasted energy and charged energy
ax.bar(df['Date'], df['DC Wasted Energy (kWh)'], color='red', label='DC Wasted Energy')
ax.bar(df['Date'], df['DC Charged Energy (kWh)'], bottom=df['DC Wasted Energy (kWh)'], color='blue', label='DC Charged Energy')

# Create legend
ax.legend()

# Set the y-label and grid
ax.set_ylabel('Energy (kWh)')
ax.grid(True)

# Set the X-axis to display dates
ax.xaxis_date()

# Set the title
plt.title('Wasted and Charged Energy During Charging Sessions')

plt.tight_layout()
plt.show()
