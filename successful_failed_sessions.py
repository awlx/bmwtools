import json
from collections import defaultdict
from fuzzywuzzy import process
import re
import datetime

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
            return original_name

    # If no match or confidence is low, consider this a new provider
    known_providers.append(provider_name_cleaned)  # Store the cleaned name for matching
    original_provider_names[provider_name_cleaned] = provider_name  # Store the original name
    return provider_name

def process_sessions(data, start_date=None, end_date=None):
    global total_failed_sessions, total_successful_sessions
    for session in data:
        # Convert timestamps to datetime objects
        session_start_time = datetime.datetime.fromtimestamp(session['startTime'])
        
        # Filter by date range if provided
        if start_date and end_date:
            if not (start_date <= session_start_time <= end_date):
                continue

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

def get_session_stats(data, start_date=None, end_date=None):
    total_sessions = 0
    total_failed_sessions = 0
    total_successful_sessions = 0
    top_failed_providers = {}
    top_successful_providers = {}

    for session in data:
        # Filter by date range if provided
        session_start_time = session['start_time']
        if start_date and end_date:
            if not (start_date <= session_start_time <= end_date):
                continue

        total_sessions += 1

        soc = session['soc_end']
        start_soc = session['soc_start']

        if soc == start_soc:
            total_failed_sessions += 1
            provider = session['provider']
            provider = fuzzy_normalize_provider_name(provider)
            top_failed_providers[provider] = top_failed_providers.get(provider, 0) + 1
            continue

        total_successful_sessions += 1
        provider = session['provider']
        provider = fuzzy_normalize_provider_name(provider)
        top_successful_providers[provider] = top_successful_providers.get(provider, 0) + 1

    # Exclude "Unknown" providers from the top 5 lists
    top_failed_providers = {k: v for k, v in top_failed_providers.items() if k != 'Unknown'}
    top_successful_providers = {k: v for k, v in top_successful_providers.items() if k != 'Unknown'}

    return {
        'total_sessions': total_sessions,
        'total_failed_sessions': total_failed_sessions,
        'total_successful_sessions': total_successful_sessions,
        'top_failed_providers': sorted(top_failed_providers.items(), key=lambda x: x[1], reverse=True)[:5],
        'top_successful_providers': sorted(top_successful_providers.items(), key=lambda x: x[1], reverse=True)[:5]
    }
