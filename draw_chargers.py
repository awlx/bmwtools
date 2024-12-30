import json
import folium
from collections import defaultdict
from geopy.distance import geodesic
from geopy.point import Point
from datetime import datetime

def load_data(file_path):
    """Load JSON data from the specified file path."""
    with open(file_path, 'r') as file:
        return json.load(file)

def find_close_location(lat, lon, locations, threshold=0.10):
    """Find if a location is close to any existing location within a threshold distance."""
    for loc in locations:
        existing_lat, existing_lon, _ = loc
        if geodesic((lat, lon), (existing_lat, existing_lon)).km < threshold:
            return loc
    return None

def process_charging_data(charging_data, start_date=None, end_date=None):
    """Process charging data to count frequencies and identify failed locations."""
    location_counts = defaultdict(int)
    failed_locations = defaultdict(int)
    locations = []

    for entry in charging_data:
        loc = entry.get("chargingLocation", {})
        latitude = loc.get("mapMatchedLatitude")
        longitude = loc.get("mapMatchedLongitude")
        address = loc.get("formattedAddress")
        session_energy = entry.get('energyConsumedFromPowerGridKwh', 0)
        session_start_time = datetime.fromtimestamp(entry.get('startTime'))

        if start_date and end_date:
            if not (start_date <= session_start_time <= end_date):
                continue

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
    
    return locations, location_counts, failed_locations

def get_marker_color(frequency, is_failed):
    """Determine marker color based on frequency and failure status."""
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

def calculate_map_center(locations):
    """Calculate the geographic center (centroid) of the map based on all locations."""
    if not locations:
        return [0, 0]
    
    # Convert locations to geopy Points
    points = [Point(lat, lon) for lat, lon, addr in locations]
    
    # Calculate centroid
    centroid = Point(sum(p.latitude for p in points) / len(points), sum(p.longitude for p in points) / len(points))
    
    return [centroid.latitude, centroid.longitude]

def create_map_base(locations, location_counts, failed_locations):
    """Create a base map with successful and failed charging locations."""
    if not locations:
        print("No locations to map.")
        return None

    # Center the map around the geographic center of all charging points
    map_center = calculate_map_center(locations)
    
    # Create the map with a custom tile server
    charging_map = folium.Map(
        location=map_center,
        zoom_start=5,
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

    return charging_map

def create_map(locations, location_counts, failed_locations):
    """Create a map with successful and failed charging locations and save to an HTML file."""
    charging_map = create_map_base(locations, location_counts, failed_locations)
    if charging_map:
        map_file_path = 'charging_points_map.html'
        charging_map.save(map_file_path)
        print(f"Map saved to {map_file_path}")

def create_map_string(charging_data, start_date=None, end_date=None):
    """Create a map with successful and failed charging locations and return as HTML string."""
    locations, location_counts, failed_locations = process_charging_data(charging_data, start_date, end_date)
    charging_map = create_map_base(locations, location_counts, failed_locations)
    return charging_map._repr_html_() if charging_map else ""

def main():
    """Main function to load data, process it, and create the map."""
    file_path = 'path_to_your_json_file.json'
    charging_data = load_data(file_path)
    start_date = datetime(2021, 1, 1)
    end_date = datetime(2021, 12, 31)
    locations, location_counts, failed_locations = process_charging_data(charging_data, start_date, end_date)
    create_map(locations, location_counts, failed_locations)

if __name__ == "__main__":
    main()