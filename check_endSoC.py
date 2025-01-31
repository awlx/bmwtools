import json

def calculate_soc_statistics(data):
    above_80_count = 0
    exactly_100_count = 0
    below_80_count = 0
    exactly_80_count = 0
    total_sessions = 0
    failed_sessions = 0

    for session in data:
        total_sessions += 1

        soc = session['soc_end']
        start_soc = session['soc_start']

        if soc == start_soc:
            failed_sessions += 1
            continue

        if soc < 80:
            below_80_count += 1
        elif soc == 80:
            exactly_80_count += 1
        elif soc > 80:
            above_80_count += 1
        if soc == 100:
            exactly_100_count += 1

    return {
        'total_sessions': total_sessions,
        'failed_sessions': failed_sessions,
        'below_80_count': below_80_count,
        'exactly_80_count': exactly_80_count,
        'above_80_count': above_80_count,
        'exactly_100_count': exactly_100_count
    }

if __name__ == "__main__":
    # Load the JSON data
    with open('./path_to_your_json_file.json') as f:
        data = json.load(f)

    # Calculate SOC statistics
    statistics = calculate_soc_statistics(data)

    # Display the results
    print(f"Total number of sessions: {statistics['total_sessions']}")
    print(f"Number of failed sessions: {statistics['failed_sessions']}")
    print(f"Number of valid sessions: {statistics['total_sessions'] - statistics['failed_sessions']}")
    print(f"Number of sessions where SOC was below 80%: {statistics['below_80_count']}")
    print(f"Number of sessions where SOC was exactly 80%: {statistics['exactly_80_count']}")
    print(f"Number of sessions where SOC was above 80%: {statistics['above_80_count']}")
    print(f"Number of sessions where SOC was exactly 100%: {statistics['exactly_100_count']}")
