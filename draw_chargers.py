import json
import folium
from collections import defaultdict
from geopy.distance import geodesic

# Load the JSON data
file_path = 'path_to_your_json_file.json'
with open(file_path, 'r') as file:
    charging_data = json.load(file)

# Extract relevant data for mapping and count frequencies
location_counts = defaultdict(int)
locations = []
failed_locations = defaultdict(int)

# Define a function to find if a location is close to any existing location
def find_close_location(lat, lon, locations, threshold=0.10):
    for loc in locations:
        existing_lat, existing_lon, _ = loc
        if geodesic((lat, lon), (existing_lat, existing_lon)).km < threshold:
            return loc
    return None

for entry in charging_data:
    loc = entry.get("chargingLocation", {})
    latitude = loc.get("mapMatchedLatitude")
    longitude = loc.get("mapMatchedLongitude")
    address = loc.get("formattedAddress")
    session_energy = entry.get('energyConsumedFromPowerGridKwh', 0)  # Total energy for session
    
    if latitude and longitude:
        location_key = (latitude, longitude, address)
        if session_energy == 0:
            close_location = find_close_location(latitude, longitude, failed_locations.keys())
            if close_location:
                failed_locations[close_location] += 1
            else:
                failed_locations[location_key] += 1
        else:
            close_location = find_close_location(latitude, longitude, locations)
            if close_location:
                location_counts[close_location] += 1
            else:
                location_counts[location_key] += 1
                locations.append(location_key)

# Function to determine marker color based on frequency
def get_marker_color(frequency, is_failed):
    if is_failed:
        return 'blue'
    if frequency > 20:
        return 'darkred'
    elif frequency > 15:
        return 'red'
    elif frequency > 10:
        return 'orange'
    elif frequency > 5:
        return 'lightred'
    else:
        return 'green'

# Create a map centered around the first charging point
if locations:
    first_location = locations[0]
    map_center = [first_location[0], first_location[1]]
    
    # Specify the custom map server URL
    charging_map = folium.Map(
        location=map_center,
        zoom_start=7,
        tiles="https://tiles.ext.ffmuc.net/osm/{z}/{x}/{y}.png",
        attr='Map data © OpenStreetMap contributors, Tiles © FFMUC'
    )

    # Create feature groups for successful and failed locations
    successful_group = folium.FeatureGroup(name='Successful Locations')
    failed_group = folium.FeatureGroup(name='Failed Locations')

    # Add successful charging points to the map with colored markers
    for lat, lon, addr in locations:
        location_key = (lat, lon, addr)
        freq = location_counts[location_key]
        is_failed = location_key in failed_locations and location_counts[location_key] == 0
        folium.Marker(
            location=[lat, lon],
            popup=f"{addr} (Visits: {freq})",
            icon=folium.Icon(color=get_marker_color(freq, is_failed))
        ).add_to(successful_group if not is_failed else failed_group)

    # Add failed charging points to the map with blue markers
    for lat, lon, addr in failed_locations:
        if location_counts[(lat, lon, addr)] == 0:  # Only add if no successful sessions
            folium.Marker(
                location=[lat, lon],
                popup=f"{addr} (Failed Visits: {failed_locations[(lat, lon, addr)]})",
                icon=folium.Icon(color='blue')
            ).add_to(failed_group)

    # Add feature groups to the map
    successful_group.add_to(charging_map)
    failed_group.add_to(charging_map)

    # Add layer control to toggle between successful and failed locations
    folium.LayerControl().add_to(charging_map)

    # Save the map to an HTML file
    map_file_path = 'charging_points_map.html'
    charging_map.save(map_file_path)
    print(f"Map saved to {map_file_path}")
else:
    print("No locations to map.")