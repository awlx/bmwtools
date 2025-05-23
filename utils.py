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
    using_estimated_values = False
    for session in data:
        try:
            start_time = datetime.datetime.fromtimestamp(session['startTime'])
            end_time = datetime.datetime.fromtimestamp(session['endTime'])
            soc_start = session['displayedStartSoc']
            soc_end = session['displayedSoc']
            energy_from_grid = session['energyConsumedFromPowerGridKwh']
            cost = session.get('chargingCostInformation', {}).get('calculatedChargingCost', 0)
            
            # Check if energyIncreaseHvbKwh exists in the data
            energy_increase_hvb = session.get('energyIncreaseHvbKwh')
            if energy_increase_hvb is None:
                # Determine if it's DC or AC charging based on average power
                # Calculate average power first
                avg_power = sum([block.get('averagePowerGridKw', 0) for block in session.get('chargingBlocks', [])]) / max(len(session.get('chargingBlocks', [])), 1)
                
                # Use different efficiency estimates based on charging type
                if avg_power >= 12:  # DC charging (typically >= 12kW)
                    energy_increase_hvb = energy_from_grid * 0.98  # 98% efficiency for DC
                else:  # AC charging
                    energy_increase_hvb = energy_from_grid * 0.92  # 92% efficiency for AC
                using_estimated_values = True
            
            efficiency = energy_increase_hvb / energy_from_grid if energy_from_grid else 0
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
                'energy_added_hvb': energy_increase_hvb,
                'cost': cost,
                'efficiency': efficiency,
                'location': location,
                'latitude': latitude,
                'longitude': longitude,
                'avg_power': avg_power,
                'grid_power_start': grid_power_start,
                'mileage': mileage,
                'session_time_minutes': session_time_minutes,
                'provider': provider,  # Add provider name
                'using_estimated_energy': energy_increase_hvb != session.get('energyIncreaseHvbKwh')
            })
        except KeyError:
            continue
    return sessions, using_estimated_values

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
        # Check if this is the SoH plot (battery capacity)
        if "Battery Capacity" in title or "SoH" in title:
            # For SoH plots, create a proper trend line by averaging groups of 10 points
            
            if len(y) > 0:
                # Create pairs of x, y data and sort them by date
                data_pairs = list(zip(x, y))
                data_pairs.sort(key=lambda pair: pair[0])
                
                # Split the data into chunks of 10 points (or fewer for the last chunk)
                chunk_size = 10
                chunks = [data_pairs[i:i+chunk_size] for i in range(0, len(data_pairs), chunk_size)]
                
                # Calculate average for each chunk
                chunk_averages = []
                for chunk in chunks:
                    chunk_x = [pair[0] for pair in chunk]
                    chunk_y = [pair[1] for pair in chunk]
                    avg_x = sum([pd.Timestamp(xi).timestamp() for xi in chunk_x]) / len(chunk_x)
                    avg_y = sum(chunk_y) / len(chunk_y)
                    chunk_averages.append((pd.Timestamp.fromtimestamp(avg_x), avg_y))
                
                # Print debug info
                #print(f"DEBUG: Created {len(chunks)} chunks from {len(y)} data points")
                #print(f"DEBUG: Chunk averages: {[round(avg[1], 2) for avg in chunk_averages]}")
                
                # Now create a line connecting these average points
                if len(chunk_averages) >= 2:
                    # Sort the averages by date (should already be sorted but just to be safe)
                    chunk_averages.sort(key=lambda pair: pair[0])
                    
                    # Extract x and y values from the averages
                    avg_x = [pair[0] for pair in chunk_averages]
                    avg_y = [pair[1] for pair in chunk_averages]
                    
                    # Create a trend line that goes through these average points
                    # We'll use simple linear interpolation between the points
                    interp_x = x  # Use original x values for smooth curve
                    interp_y = []
                    
                    # For each original x value, interpolate y based on the chunk averages
                    for xi in interp_x:
                        xi_ts = pd.Timestamp(xi).timestamp()
                        
                        # Find the two neighboring chunk averages
                        # Default to the first or last chunk average if outside the range
                        if xi_ts <= pd.Timestamp(avg_x[0]).timestamp():
                            interp_y.append(avg_y[0])
                        elif xi_ts >= pd.Timestamp(avg_x[-1]).timestamp():
                            interp_y.append(avg_y[-1])
                        else:
                            # Find the two neighboring points for interpolation
                            for i in range(len(avg_x) - 1):
                                x1_ts = pd.Timestamp(avg_x[i]).timestamp()
                                x2_ts = pd.Timestamp(avg_x[i + 1]).timestamp()
                                
                                if x1_ts <= xi_ts <= x2_ts:
                                    # Linear interpolation
                                    y1 = avg_y[i]
                                    y2 = avg_y[i + 1]
                                    ratio = (xi_ts - x1_ts) / (x2_ts - x1_ts)
                                    interp_val = y1 + ratio * (y2 - y1)
                                    interp_y.append(interp_val)
                                    break
                    
                    smoothed_values = interp_y
                else:
                    # Not enough chunks, use simple average
                    simple_mean = sum(y) / len(y)
                    smoothed_values = [simple_mean for _ in x]
        else:
            # For other plots, use the original rolling window approach
            window_size = max(1, min(20, len(y)))  # Ensure window size is at least 1
            smoothed_values = pd.Series(y).rolling(window=window_size, min_periods=1).mean()
            
        fig.add_trace(go.Scatter(
            x=x,
            y=smoothed_values,
            mode='lines',
            line=dict(color='red', width=2),  # Make line thicker for better visibility
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
