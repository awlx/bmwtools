import hashlib
import datetime
import plotly.graph_objs as go
import pandas as pd
from folium import Map, Marker
import os

# Function to calculate the sha256sum of a file
def calculate_sha256(file_path):
    try:
        with open(file_path, 'rb') as f:
            file_content = f.read()
        return hashlib.sha256(file_content).hexdigest()
    except Exception as e:
        return f"Error calculating hash: {e}"

# Function to get the SHA256 hash of all Python files in the repository
def get_all_files_sha256(directory):
    sha256_hashes = {}
    for root, _, files in os.walk(directory):
        for file in files:
            if file.endswith('.py'):
                file_path = os.path.join(root, file)
                sha256_hashes[file] = calculate_sha256(file_path)
    return sha256_hashes

# Add the SHA256 hash of all Python files in the repository to the disclaimer
def get_disclaimer_with_hash():
    sha256_hashes = get_all_files_sha256(os.path.dirname(__file__))
    sorted_hashes = sorted(sha256_hashes.items())  # Sort by file name alphabetically
    hash_lines = "\n".join([f"{file}: {hash}" for file, hash in sorted_hashes])
    return ('Disclaimer: This application stores all uploaded data in memory, if you refresh your session is lost.\n' 
            'CarData contains location data of your charges. Use at your own risk!\n'
            f"SHA256 of the files:\n{hash_lines}\n" 
            'You can verify authenticity at https://github.com/awlx/bmwtools')

# Function to process JSON data
def process_data(data):
    sessions = []
    for session in data:
        try:
            start_time = datetime.datetime.fromtimestamp(session['startTime'])
            end_time = datetime.datetime.fromtimestamp(session['endTime'])
            soc_start = session['displayedStartSoc']
            soc_end = session['displayedSoc']
            energy_from_grid = session['energyConsumedFromPowerGridKwh']
            cost = session.get('chargingCostInformation', {}).get('calculatedChargingCost', 0)
            efficiency = session.get('energyIncreaseHvbKwh', 0) / energy_from_grid if energy_from_grid else 0
            location = session.get('chargingLocation', {}).get('formattedAddress', 'Unknown Location')
            latitude = session.get('chargingLocation', {}).get('mapMatchedLatitude', 0)
            longitude = session.get('chargingLocation', {}).get('mapMatchedLongitude', 0)
            avg_power = sum([block.get('averagePowerGridKw', 0) for block in session.get('chargingBlocks', [])]) / max(len(session.get('chargingBlocks', [])), 1)
            grid_power_start = [block.get('averagePowerGridKw', 0) for block in session.get('chargingBlocks', [])]
            mileage = session.get('mileage', 0)
            session_time_minutes = (end_time - start_time).total_seconds() / 60
            provider = session.get('publicChargingPoint', {}).get('potentialChargingPointMatches', [{}])[0].get('providerName', 'Unknown')

            sessions.append({
                'start_time': start_time,
                'end_time': end_time,
                'soc_start': soc_start,
                'soc_end': soc_end,
                'energy_from_grid': energy_from_grid,
                'energy_added_hvb': session.get('energyIncreaseHvbKwh', 0),
                'cost': cost,
                'efficiency': efficiency,
                'location': location,
                'latitude': latitude,
                'longitude': longitude,
                'avg_power': avg_power,
                'grid_power_start': grid_power_start,
                'mileage': mileage,
                'session_time_minutes': session_time_minutes,
                'provider': provider  # Add provider name
            })
        except KeyError:
            continue
    return sessions

# Function to calculate estimated battery capacity (SoH)
def calculate_estimated_battery_capacity(sessions):
    estimated_battery_capacity = []
    for session in sessions:
        if session['energy_added_hvb'] >= 30:
            soc_change = session['soc_end'] - session['soc_start']
            if soc_change != 0:
                estimated_capacity = (session['energy_added_hvb'] * 100) / soc_change
            else:
                estimated_capacity = 0
            estimated_battery_capacity.append({
                'date': session['start_time'],
                'estimated_battery_capacity': estimated_capacity,
                'soc_change': soc_change
            })
    return estimated_battery_capacity

# Function to calculate overall efficiency and power consumption
def calculate_overall_stats(sessions):
    total_energy_added = sum(s['energy_added_hvb'] for s in sessions)
    total_energy_from_grid = sum(s['energy_from_grid'] for s in sessions)
    if sessions:
        total_distance = max(s['mileage'] for s in sessions) - min(s['mileage'] for s in sessions)
    else:
        total_distance = max(s['mileage'] for s in sessions) if sessions else 0
    
    overall_efficiency = (total_energy_added / total_energy_from_grid) if total_energy_from_grid else 0
    power_consumption_per_100km = (total_energy_from_grid / total_distance) * 100 if total_distance else 0
    power_consumption_per_100km_without_grid_losses = (total_energy_added / total_distance) * 100 if total_distance else 0
    
    return overall_efficiency, power_consumption_per_100km, power_consumption_per_100km_without_grid_losses

# Function to create a gauge trace
def create_gauge_trace(value, title, color, domain_x, domain_y=[0, 1], range_max=None):
    return go.Indicator(
        mode="gauge+number",
        value=value,
        title={'text': title, 'font': {'size': 14}},  # Adjust the font size here
        domain={'x': domain_x, 'y': domain_y},
        gauge={'axis': {'range': [0, range_max] if range_max else [None, None]}, 'bar': {'color': color}}
    )

# Function to create a scatter plot with optional trend line and labels
def create_scatter_plot(x, y, title, xaxis_title, yaxis_title, color='blue', mode='markers', size=10, trendline=False):
    fig = go.Figure()
    fig.add_trace(go.Scatter(
        x=x,
        y=y,
        mode=mode,
        marker=dict(size=size, color=color),
        name=title
    ))
    if trendline:
        window_size = max(1, min(20, len(y)))  # Ensure window size is at least 1
        fig.add_trace(go.Scatter(
            x=x,
            y=pd.Series(y).rolling(window=window_size, min_periods=1).mean(),  # Using rolling mean for trendline
            mode='lines',
            line=dict(color='red'),
            name='Trend'
        ))
    # Add labels to the beginning and end of the graph
    if x and y:
        fig.add_trace(go.Scatter(
            x=[x[0], x[-1]],
            y=[y[0], y[-1]],
            mode='text',
            text=[f"{y[0]:.2f}", f"{y[-1]:.2f}"],
            textposition='top center',
            showlegend=False  # Remove legend item for labels
        ))
    fig.update_layout(
        title=title,
        xaxis_title=xaxis_title,
        yaxis_title=yaxis_title,
        template='plotly_white'
    )
    return fig

# Function to create a Folium map
def create_folium_map(sessions, selected_session=None):
    if selected_session:
        zoom_level = 13
        center = [selected_session['latitude'], selected_session['longitude']]
    else:
        zoom_level = 5
        center = [sessions[0]['latitude'], sessions[0]['longitude']] if sessions else [0, 0]

    m = Map(location=center, zoom_start=zoom_level, tiles="https://tiles.ext.ffmuc.net/osm/{z}/{x}/{y}.png", attr="OpenStreetMap")
    for session in sessions:
        Marker([session['latitude'], session['longitude']], popup=session['location']).add_to(m)
    map_html = m._repr_html_()
    return map_html
