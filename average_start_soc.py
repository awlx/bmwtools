import json
from collections import defaultdict
import re

# Load the JSON data
with open('./path_to_your_json_file.json') as f:
    data = json.load(f)

# Initialize lists to store startSoc values for successful DC and AC charging sessions
start_soc_dc_successful = []
start_soc_ac_successful = []


# Loop through each session
for session in data:
    # Check if 'displayedSoc' and 'displayedStartSoc' exist in the session
    if 'displayedSoc' in session and 'displayedStartSoc' in session:
        soc = session['displayedSoc']
        start_soc = session['displayedStartSoc']
        
        # Check if 'chargingBlocks' exist and is not empty
        if 'chargingBlocks' in session and session['chargingBlocks']:
            # Get the max charging power from the charging blocks
            max_power = max([block['averagePowerGridKw'] for block in session['chargingBlocks']])
            
            # Check for successful charging (startSoc < displayedSoc)
            if start_soc < soc:
                # DC charging (max power greater than 11kW)
                if max_power > 11:
                    start_soc_dc_successful.append(start_soc)
                # AC charging (max power 11kW or less)
                elif max_power <= 11:
                    start_soc_ac_successful.append(start_soc)

# Function to calculate and display statistics
def display_stats(start_soc_list, charging_type):
    if start_soc_list:
        average_start_soc = sum(start_soc_list) / len(start_soc_list)
        highest_start_soc = max(start_soc_list)
        lowest_start_soc = min(start_soc_list)

        # Output the results
        print(f"\nAverage startSoC for successful {charging_type} charging sessions: {average_start_soc:.2f}")
        print(f"Highest startSoC for successful {charging_type} charging sessions: {highest_start_soc}")
        print(f"Lowest startSoC for successful {charging_type} charging sessions: {lowest_start_soc}")
    else:
        print(f"\nNo successful {charging_type} charging sessions found.")

# Display statistics for both DC and AC charging
display_stats(start_soc_dc_successful, "DC")
display_stats(start_soc_ac_successful, "AC")
