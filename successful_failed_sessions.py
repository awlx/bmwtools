import json
from collections import defaultdict
from fuzzywuzzy import process
import re

# Load the JSON data
with open('./path_to_your_json_file.json') as f:
    data = json.load(f)

# Initialize dictionaries to store the count of failed and successful sessions per provider
failed_providers_count = defaultdict(int)
successful_providers_count = defaultdict(int)

total_failed_sessions = 0     # To track the total number of failed sessions
total_successful_sessions = 0  # To track the total number of successful sessions

# This will store unique provider names we've seen so far in their original form
original_provider_names = {}
known_providers = []  # This will store the processed (simplified) provider names for fuzzy matching

# Function to clean provider names by removing extraneous terms like "HPC", "DC", "GmbH", etc.
def preprocess_provider_name(provider_name):
    # Remove terms like "HPC", "DC", "AC", "GmbH" etc. for better matching
    cleaned_name = re.sub(r'\b(HPC|DC|AC|GmbH)\b', '', provider_name, flags=re.IGNORECASE)
    # Strip extra spaces that might occur after removing words
    cleaned_name = re.sub(r'\s+', ' ', cleaned_name).strip()
    return cleaned_name.lower()

# Function to use fuzzy matching for provider names (after cleaning)
def fuzzy_normalize_provider_name(provider_name):
    # Clean the provider name for matching purposes
    provider_name_cleaned = preprocess_provider_name(provider_name)

    # If there are known providers already, try to match using fuzzy matching
    if known_providers:
        best_match_cleaned, confidence = process.extractOne(provider_name_cleaned, known_providers)
        # If the confidence is high enough, return the original version of the matched provider name
        if confidence > 90:  # 90% confidence threshold for matching
            original_name = original_provider_names[best_match_cleaned]  # Get the original case name
            print(f"Matched '{provider_name}' with '{original_name}' (confidence: {confidence})")  # Debug output
            return original_name

    # If no match or confidence is low, consider this a new provider
    print(f"New provider: '{provider_name}' added to known providers")  # Debug output
    known_providers.append(provider_name_cleaned)  # Store the cleaned name for matching
    original_provider_names[provider_name_cleaned] = provider_name  # Store the original name
    return provider_name

# Loop through each session
for session in data:
    # Check if 'displayedSoc' and 'displayedStartSoc' exist in the session
    if 'displayedSoc' in session and 'displayedStartSoc' in session:
        soc = session['displayedSoc']
        start_soc = session['displayedStartSoc']
        
        # Check if 'publicChargingPoint' and 'providerName' exist
        if 'publicChargingPoint' in session and 'potentialChargingPointMatches' in session['publicChargingPoint']:
            # Extract the providerName (assuming first match is relevant)
            provider_name = session['publicChargingPoint']['potentialChargingPointMatches'][0].get('providerName', 'Unknown')
            
            # Normalize provider name using fuzzy matching (case-insensitive but stores the original name)
            provider_name = fuzzy_normalize_provider_name(provider_name)
            
            # Check for failed sessions (start SOC equals end SOC)
            if soc == start_soc:
                total_failed_sessions += 1
                failed_providers_count[provider_name] += 1
            else:  # Successful sessions
                total_successful_sessions += 1
                successful_providers_count[provider_name] += 1

# Sort the providers by the number of failed and successful sessions in descending order
sorted_failed_providers = sorted(failed_providers_count.items(), key=lambda item: item[1], reverse=True)
sorted_successful_providers = sorted(successful_providers_count.items(), key=lambda item: item[1], reverse=True)

# Display the provider names and their corresponding number of failed sessions
if sorted_failed_providers:
    print("\nNumber of failed sessions per provider (sorted by failed sessions):")
    for provider, count in sorted_failed_providers:
        print(f"{provider}: {count} failed sessions")
else:
    print("No failed sessions found.")

# Output the total number of failed sessions
print(f"\nTotal number of failed sessions: {total_failed_sessions}")

# Display the provider names and their corresponding number of successful sessions
if sorted_successful_providers:
    print("\nNumber of successful sessions per provider (sorted by successful sessions):")
    for provider, count in sorted_successful_providers:
        print(f"{provider}: {count} successful sessions")
else:
    print("No successful sessions found.")

# Output the total number of successful sessions
print(f"\nTotal number of successful sessions: {total_successful_sessions}")
