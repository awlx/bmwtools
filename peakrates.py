import json
import pandas as pd
import plotly.express as px

# Load the JSON file
with open('path_to_your_json_file.json', 'r') as file:
    data = json.load(file)

# Extract the highest charging rate (above 180 kW) for each session and the corresponding date
charge_dates = []
peak_rates = []

for entry in data:
    # Filter charging blocks where the averagePowerGridKw is >= 180
    filtered_blocks = [block for block in entry['chargingBlocks'] if block['averagePowerGridKw'] >= 100]
    
    if filtered_blocks:  # Proceed if there are any blocks left after filtering
        # Find the highest averagePowerGridKw in the filtered blocks
        peak_rate = max(block['averagePowerGridKw'] for block in filtered_blocks)
        # Use the start time of the first block in the session as the session date
        start_time = pd.to_datetime(filtered_blocks[0]['startTime'], unit='s')
        charge_dates.append(start_time)
        peak_rates.append(peak_rate)

# Create a DataFrame using the extracted data
df = pd.DataFrame({
    'Date': charge_dates,
    'Peak Power (kW)': peak_rates
})

# Sort the data by date
df = df.sort_values(by='Date')

# Create a line plot using Plotly
fig = px.line(df, x='Date', y='Peak Power (kW)', 
              title='Peak Charge Rates Over Time (Filtered: Avg Power ≥ 160 kW)', 
              labels={'Date': 'Date', 'Peak Power (kW)': 'Peak Power (kW)'},
              markers=True)

# Add a vertical line for September 2024
fig.add_shape(
    type="line",
    x0="2024-09-01", x1="2024-09-01",
    y0=0, y1=1,
    line=dict(color="Red", width=2, dash="dash")
)

# Show the plot
fig.show()
