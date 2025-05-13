from dash import Input, Output, State, html
import json
import base64
import datetime
import plotly.graph_objs as go
from concurrent.futures import ThreadPoolExecutor
from utils import process_data, create_gauge_trace, calculate_overall_stats, create_scatter_plot, calculate_estimated_battery_capacity, create_folium_map
from successful_failed_sessions import get_session_stats
from draw_chargers import create_map_string
from check_endSoC import calculate_soc_statistics

def register_callbacks(app):
    @app.callback(
        [Output('session-dropdown', 'options'),
         Output('session-dropdown', 'value'),
         Output('total-energy-gauge', 'figure'),
         Output('current-km-gauge', 'figure'),
         Output('session-data', 'data'),
         Output('total-sessions-gauge', 'figure'),
         Output('failed-sessions-gauge', 'figure'),
         Output('successful-sessions-gauge', 'figure'),
         Output('top-failed-providers', 'children'),
         Output('top-successful-providers', 'children'),
         Output('charging-locations-map', 'srcDoc'),
         Output('overall-efficiency-gauge', 'figure'),
         Output('power-consumption-gauge', 'figure'),
         Output('power-consumption-without-grid-losses-gauge', 'figure'),
         Output('soc-stats', 'children'),
         Output('energy-data-warning', 'children'),
         Output('energy-data-warning', 'style')],
        [Input('upload-json', 'contents'),
         Input('load-demo-data', 'n_clicks'),
         Input('date-picker-range', 'start_date'),
         Input('date-picker-range', 'end_date'),
         Input('toggle-units', 'n_clicks')]
    )
    def upload_json(contents, n_clicks, start_date, end_date, toggle_units):
        if contents:
            content_type, content_string = contents.split(',')
            decoded = base64.b64decode(content_string)
            try:
                data = json.loads(decoded)
            except json.JSONDecodeError:
                return [], None, {}, {}, [], {}, {}, {}, [], "", "", {}, {}, {}, ""
        elif n_clicks > 0:
            with open('FINAL_DEMO_CHARGING_DATA_SMOOTH_CURVES.JSON', 'r') as f:
                data = json.load(f)
        else:
            return [], None, {}, {}, [], {}, {}, {}, [], "", "", {}, {}, {}, ""

        sessions, using_estimated_values = process_data(data)

        # Filter sessions by date range if selected
        if start_date and end_date:
            start_date = datetime.datetime.fromisoformat(start_date)
            end_date = datetime.datetime.fromisoformat(end_date)
            sessions = [s for s in sessions if start_date <= s['start_time'] <= end_date]

        options = [
            {'label': f"{s['start_time'].strftime('%Y-%m-%d %H:%M')} - {s['location']}", 'value': i}
            for i, s in enumerate(sessions)
        ]

        def calculate_total_energy():
            total_energy_dc = sum(s['energy_added_hvb'] for s in sessions if s['avg_power'] >= 12)
            total_energy_ac = sum(s['energy_added_hvb'] for s in sessions if s['avg_power'] < 12)
            total_energy_fig = go.Figure()
            total_energy_fig.add_trace(create_gauge_trace(total_energy_dc, "Total DC Energy (kWh)", "blue", [0, 0.28], range_max=total_energy_dc + total_energy_ac + 10))
            total_energy_fig.add_trace(create_gauge_trace(total_energy_ac, "Total AC Energy (kWh)", "green", [0.36, 0.64], range_max=total_energy_dc + total_energy_ac + 10))
            total_energy_fig.add_trace(create_gauge_trace(total_energy_dc + total_energy_ac, "Total Energy (AC + DC)", "purple", [0.72, 1], range_max=total_energy_dc + total_energy_ac + 20))
            total_energy_fig.update_layout(height=400, width=900, template='plotly_white')
            return total_energy_fig

        def calculate_current_km():
            if start_date and end_date:
                current_km = max(s['mileage'] for s in sessions) - min(s['mileage'] for s in sessions) if sessions else 0
            else:
                current_km = max(s['mileage'] for s in sessions) if sessions else 0
            current_km_fig = go.Figure()
            current_km_fig.add_trace(create_gauge_trace(current_km, "Driven km", "orange", [0, 1], range_max=current_km))
            current_km_fig.update_layout(height=400, width=300, template='plotly_white')
            return current_km_fig

        def calculate_session_stats():
            return get_session_stats(data=sessions, start_date=start_date, end_date=end_date)

        def calculate_map_html():
            return create_map_string(data, start_date, end_date)

        def calculate_overall_efficiency():
            return calculate_overall_stats(sessions)

        def calculate_soc_stats():
            return calculate_soc_statistics(sessions)

        with ThreadPoolExecutor() as executor:
            total_energy_future = executor.submit(calculate_total_energy)
            current_km_future = executor.submit(calculate_current_km)
            session_stats_future = executor.submit(calculate_session_stats)
            map_html_future = executor.submit(calculate_map_html)
            overall_efficiency_future = executor.submit(calculate_overall_efficiency)
            soc_stats_future = executor.submit(calculate_soc_stats)

            total_energy_fig = total_energy_future.result()
            current_km_fig = current_km_future.result()
            session_stats = session_stats_future.result()
            map_html_content = map_html_future.result()
            overall_efficiency, power_consumption_per_100km, power_consumption_per_100km_without_grid_losses = overall_efficiency_future.result()
            soc_stats_data = soc_stats_future.result()

        total_sessions_fig = go.Figure()
        failed_sessions_fig = go.Figure()
        successful_sessions_fig = go.Figure()
        total_sessions_fig.add_trace(create_gauge_trace(session_stats['total_sessions'], "Total Sessions", "blue", [0, 1], range_max=session_stats['total_sessions']))
        failed_sessions_fig.add_trace(create_gauge_trace(session_stats['total_failed_sessions'], "Failed Sessions", "red", [0, 1], range_max=session_stats['total_sessions']))
        successful_sessions_fig.add_trace(create_gauge_trace(session_stats['total_successful_sessions'], "Successful Sessions", "green", [0, 1], range_max=session_stats['total_sessions']))
        total_sessions_fig.update_layout(height=300, width=300, template='plotly_white')
        failed_sessions_fig.update_layout(height=300, width=300, template='plotly_white')
        successful_sessions_fig.update_layout(height=300, width=300, template='plotly_white')
        top_failed_providers = [html.Li(f"{provider}: {count} failed sessions") for provider, count in session_stats['top_failed_providers']]
        top_successful_providers = [html.Li(f"{provider}: {count} successful sessions") for provider, count in session_stats['top_successful_providers']]

        overall_efficiency_fig = go.Figure()
        overall_efficiency_fig.add_trace(create_gauge_trace(overall_efficiency * 100, "Overall Efficiency (%)", "blue", [0, 1], range_max=100))
        overall_efficiency_fig.update_layout(height=300, width=300, template='plotly_white')
        
        # Prepare the warning message for when estimated energy values are being used
        warning_message = None
        warning_style = {'textAlign': 'center', 'color': 'orange', 'fontWeight': 'bold', 'margin': '10px', 'display': 'none'}
        
        if using_estimated_values:
            warning_message = "⚠️ Warning: Your JSON file is missing 'energyIncreaseHvbKwh' data. Energy values are estimated using 98% efficiency for DC charging and 92% efficiency for AC charging."
            warning_style['display'] = 'block'
        
        power_consumption_fig = go.Figure()
        power_consumption_fig.add_trace(create_gauge_trace(power_consumption_per_100km, "Avg Power Consumption (kWh/100km)", "green", [0, 1], range_max=power_consumption_per_100km))
        power_consumption_fig.update_layout(height=300, width=300, template='plotly_white')
        
        power_consumption_without_grid_losses_fig = go.Figure()
        power_consumption_without_grid_losses_fig.add_trace(create_gauge_trace(power_consumption_per_100km_without_grid_losses, "Avg Consumption w/o Grid Losses (kWh/100km)", "purple", [0, 1], range_max=power_consumption_per_100km))
        power_consumption_without_grid_losses_fig.update_layout(height=300, width=300, template='plotly_white')

        # Determine if units should be in km or miles
        use_miles = toggle_units % 2 == 1

        if use_miles:
            current_miles = current_km * 0.621371
            current_km_fig = go.Figure()
            current_km_fig.add_trace(create_gauge_trace(current_miles, "Driven miles", "orange", [0, 1], range_max=current_miles))
            current_km_fig.update_layout(height=400, width=300, template='plotly_white')

        soc_stats = [
            html.Li(f"Total Sessions: {soc_stats_data['total_sessions']}"),
            html.Li(f"Sessions with end SoC > 80%: {soc_stats_data['above_80_count']}"),
            html.Li(f"Sessions with end SoC = 100%: {soc_stats_data['exactly_100_count']}"),
            html.Li(f"Sessions with end SoC < 80%: {soc_stats_data['below_80_count']}"),
            html.Li(f"Sessions with end SoC = 80%: {soc_stats_data['exactly_80_count']}"),
            html.Li(f"Failed Sessions: {soc_stats_data['failed_sessions']}")
        ]

        return options, 0, total_energy_fig, current_km_fig, sessions, total_sessions_fig, failed_sessions_fig, successful_sessions_fig, top_failed_providers, top_successful_providers, map_html_content, overall_efficiency_fig, power_consumption_fig, power_consumption_without_grid_losses_fig, soc_stats, warning_message, warning_style

    @app.callback(
        [Output('charge-details-graph', 'figure'),
         Output('session-info', 'children'),
         Output('combined-gauges', 'figure'),
         Output('grid-power-graph', 'figure'),
         Output('range-map', 'srcDoc'),
         Output('overview-scatterplot', 'figure'),
         Output('average-gridpower-scatterplot', 'figure'),
         Output('estimated-battery-capacity-scatterplot', 'figure')],
        [Input('session-dropdown', 'value'),
         State('session-data', 'data')]
    )
    def update_dashboard(selected_session, sessions):
        if selected_session is None or not sessions:
            return {}, "", {}, {}, "", {}, {}, {}

        # Convert start_time and end_time back to datetime objects
        for session in sessions:
            session['start_time'] = datetime.datetime.fromisoformat(session['start_time'])
            session['end_time'] = datetime.datetime.fromisoformat(session['end_time'])

        session = sessions[selected_session]

        # Charge details graph
        charge_details_fig = create_scatter_plot(
            x=[session['start_time'], session['end_time']],
            y=[session['soc_start'], session['soc_end']],
            title='Charge Details',
            xaxis_title='Time',
            yaxis_title='SOC (%)',
            color='blue',
            mode='lines+markers'
        )

        # Session info text
        session_info = f"Energy Added: {session['energy_added_hvb']} kWh, Cost: €{session['cost']}, Efficiency: {session['efficiency']:.2%}, Location: {session['location']}"

        # Combined gauges
        combined_gauges = go.Figure()
        combined_gauges.add_trace(create_gauge_trace(session['avg_power'], "Average Grid Power (kW)", "darkblue", [0, 0.45], [0.6, 1], max([s['avg_power'] for s in sessions])))
        combined_gauges.add_trace(create_gauge_trace(session['cost'], "Cost (€)", "green", [0.55, 1], [0.6, 1], max([s['cost'] for s in sessions])))
        combined_gauges.add_trace(create_gauge_trace(session['efficiency'] * 100, "Efficiency (%)", "orange", [0, 0.45], [0.2, 0.6], 100))
        combined_gauges.add_trace(create_gauge_trace(session['energy_added_hvb'], "Energy Added (kWh)", "purple", [0.55, 1], [0.2, 0.6], max([s['energy_added_hvb'] for s in sessions])))
        combined_gauges.add_trace(create_gauge_trace(session['session_time_minutes'], "Session Time (minutes)", "red", [0.25, 0.75], [0, 0.2], max([s['session_time_minutes'] for s in sessions])))
        combined_gauges.update_layout(template='plotly_white', height=800)

        # Grid Power over Time graph
        grid_power_fig = create_scatter_plot(
            x=[session['start_time'] + datetime.timedelta(seconds=i * (session['end_time'] - session['start_time']).total_seconds() / len(session['grid_power_start'])) for i in range(len(session['grid_power_start']))],
            y=session['grid_power_start'],
            title='Grid Power Over Time',
            xaxis_title='Time',
            yaxis_title='Grid Power (kW)',
            color='green',
            mode='lines+markers'
        )

        # Check if grid_power_start is not empty
        if session['grid_power_start']:
            # Find the peak value and its corresponding time
            peak_value = max(session['grid_power_start'])
            peak_index = session['grid_power_start'].index(peak_value)
            peak_time = session['start_time'] + datetime.timedelta(seconds=peak_index * (session['end_time'] - session['start_time']).total_seconds() / len(session['grid_power_start']))

            # Add a marker for the peak value
            grid_power_fig.add_trace(go.Scatter(
                x=[peak_time],
                y=[peak_value],
                mode='markers+text',
                marker=dict(size=12, color='red', symbol='x'),
                text=[f"Peak: {peak_value:.2f} kW"],
                textposition='bottom center',
                name='Peak'
            ))

        # Overview scatterplot
        overview_fig = go.Figure()
        for s in sessions:
            overview_fig.add_trace(go.Scatter(
                x=[s['start_time']],
                y=[s['energy_added_hvb']],
                mode='markers',
                marker=dict(size=10, color='blue'),
                name=f"{s['start_time'].strftime('%Y-%m-%d %H:%M')} - {[s['energy_added_hvb']]} kWh - {s['location']}"
            ))
        overview_fig.update_layout(
            showlegend=True,
            title='Energy added per charging session',
            yaxis_title='kWh',
            xaxis_title='Date',
        )

        # Average Grid Power scatterplot
        avg_gridpower_fig = go.Figure()
        for s in sessions:
            avg_gridpower_fig.add_trace(go.Scatter(
                x=[i for i in range(len(s['grid_power_start']))],
                y=s['grid_power_start'],
                mode='lines',
                marker=dict(size=6, color=s['grid_power_start'], colorscale='Viridis', showscale=False),
                name=f"Session {s['start_time']}"
            ))
        avg_gridpower_fig.update_layout(
            title='Average Grid Power Across All Sessions',
            xaxis_title='Session Time (minutes)',
            yaxis_title='Grid Power (kW)',
            template='plotly_white',
            showlegend=False
        )

        # Estimated Battery Capacity scatterplot
        estimated_battery_capacity_data = calculate_estimated_battery_capacity(sessions)
        estimated_battery_capacity_fig = create_scatter_plot(
            x=[data['date'] for data in estimated_battery_capacity_data],
            y=[data['estimated_battery_capacity'] for data in estimated_battery_capacity_data],
            title='Estimated Battery Capacity (SoH) Over Time - Guesstimated',
            xaxis_title='Date',
            yaxis_title='kWh',
            color='red',
            trendline=True  # Add trendline
        )

        # Generate Folium map
        map_html_content = create_folium_map(sessions, session)

        return charge_details_fig, session_info, combined_gauges, grid_power_fig, map_html_content, overview_fig, avg_gridpower_fig, estimated_battery_capacity_fig
