import json

# Load the JSON data
with open('./path_to_your_json_file.json') as f:
    data = json.load(f)

# Initialize counters for SOC categories
above_80_count = 0
exactly_100_count = 0
below_80_count = 0
exactly_80_count = 0
total_sessions = 0
failed_sessions = 0

# Loop through each session
for session in data:
    # Increment total session count
    total_sessions += 1
    
    # Check if 'displayedSoc' and 'displayedStartSoc' exist in the session
    if 'displayedSoc' in session and 'displayedStartSoc' in session:
        soc = session['displayedSoc']
        start_soc = session['displayedStartSoc']
        
        # Check for failed session where start SOC is the same as end SOC
        if soc == start_soc:
            failed_sessions += 1
            continue  # Skip this session for further analysis
        
        # Count if SOC is below 80%
        if soc < 80:
            below_80_count += 1
        
        # Count if SOC is exactly 80%
        elif soc == 80:
            exactly_80_count += 1
        
        # Count if SOC is above 80%
        elif soc > 80:
            above_80_count += 1
        
        # Count if SOC is exactly 100%
        if soc == 100:
            exactly_100_count += 1

# Display the results
print(f"Total number of sessions: {total_sessions}")
print(f"Number of failed sessions: {failed_sessions}")
print(f"Number of valid sessions: {total_sessions - failed_sessions}")
print(f"Number of sessions where SOC was below 80%: {below_80_count}")
print(f"Number of sessions where SOC was exactly 80%: {exactly_80_count}")
print(f"Number of sessions where SOC was above 80%: {above_80_count}")
print(f"Number of sessions where SOC was exactly 100%: {exactly_100_count}")
