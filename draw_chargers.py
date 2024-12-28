import json
import folium

# Load the JSON data
file_path = 'path_to_your_json_file.json'
with open(file_path, 'r') as file:
    charging_data = json.load(file)

# Extract relevant data for mapping
locations = []
for entry in charging_data:
    loc = entry.get("chargingLocation", {})
    latitude = loc.get("mapMatchedLatitude")
    longitude = loc.get("mapMatchedLongitude")
    address = loc.get("formattedAddress")
    if latitude and longitude:
        locations.append((latitude, longitude, address))

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

    # Add charging points to the map
    for lat, lon, addr in locations:
        folium.Marker(location=[lat, lon], popup=addr).add_to(charging_map)

    # Save the map to an HTML file
    map_file_path = 'charging_points_map_ffmuc.html'
    charging_map.save(map_file_path)
    print(f"Map saved to {map_file_path}")
else:
    print("No locations to map.")
