import json
from collections import defaultdict

# Load the JSON data
with open('./path_to_your_json_file.json') as f:
    data = json.load(f)

# Initialize a dictionary to store the count of failed sessions for each provider
failed_providers_count = defaultdict(int)
total_failed_sessions = 0  # To track the total number of failed sessions

# Loop through each session
for session in data:
    # Check if 'displayedSoc' and 'displayedStartSoc' exist in the session
    if 'displayedSoc' in session and 'displayedStartSoc' in session:
        soc = session['displayedSoc']
        start_soc = session['displayedStartSoc']
        
        # Check for failed session where start SOC is the same as end SOC
        if soc == start_soc:
            total_failed_sessions += 1  # Increment total failed session count
            
            # Check if 'publicChargingPoint' and 'providerName' exist
            if 'publicChargingPoint' in session and 'potentialChargingPointMatches' in session['publicChargingPoint']:
                # Extract the providerName (assuming first match is relevant)
                provider_name = session['publicChargingPoint']['potentialChargingPointMatches'][0].get('providerName', 'Unknown')
     
                # Normalize provider name: Treat all variations of "Ionity" and "EnBW mobility+" as the same (case-insensitive)
                if "ionity" in provider_name.lower():
                    provider_name = "HPC/DC IONITY GmbH"
                elif "enbw mobility+" in provider_name.lower():
                    provider_name = "HPC/DC/AC EnBW mobility+"
                # Increment the failed session count for the provider
                failed_providers_count[provider_name] += 1

# Sort the providers by the number of failed sessions in descending order
sorted_failed_providers = sorted(failed_providers_count.items(), key=lambda item: item[1], reverse=True)

# Display the provider names and their corresponding number of failed sessions
if sorted_failed_providers:
    print("Number of failed sessions per provider (sorted by failed sessions):")
    for provider, count in sorted_failed_providers:
        print(f"{provider}: {count} failed sessions")
else:
    print("No failed sessions found.")

# Output the total number of failed sessions
print(f"\nTotal number of failed sessions: {total_failed_sessions}")
