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
    if latitude and longitude:
        close_location = find_close_location(latitude, longitude, locations)
        if close_location:
            location_counts[close_location] += 1
        else:
            location_key = (latitude, longitude, address)
            location_counts[location_key] += 1
            locations.append(location_key)

# Function to determine marker color based on frequency
def get_marker_color(frequency):
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

    # Add charging points to the map with colored markers
    for lat, lon, addr in locations:
        freq = location_counts[(lat, lon, addr)]
        folium.Marker(
            location=[lat, lon],
            popup=f"{addr} (Visits: {freq})",
            icon=folium.Icon(color=get_marker_color(freq))
        ).add_to(charging_map)

    # Save the map to an HTML file
    map_file_path = 'charging_points_map.html'
    charging_map.save(map_file_path)
    print(f"Map saved to {map_file_path}")
else:
    print("No locations to map.")