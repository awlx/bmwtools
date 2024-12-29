import dash
from dash import dcc, html, Input, Output, State
import plotly.graph_objs as go
import json
import datetime
import folium
from folium import Map, Marker
import base64
import io

# DISCLAIMER
# This application stores all uploaded data in memory for processing.
# Use this tool at your own risk and ensure you handle sensitive data appropriately.

# Initialize Dash app
app = dash.Dash(__name__)
app.title = 'BMW CarData - Charging Session Dashboard'

# Function to process JSON data
def process_data(data):
    sessions = []
    for session in data:
        try:
            start_time = datetime.datetime.fromtimestamp(session['startTime'])
            end_time = datetime.datetime.fromtimestamp(session['endTime'])
            soc_start = session['displayedStartSoc']
            soc_end = session['displayedSoc']
            energy_added = session['energyConsumedFromPowerGridKwh']
            cost = session.get('chargingCostInformation', {}).get('calculatedChargingCost', 0)
            efficiency = session.get('energyIncreaseHvbKwh', 0) / energy_added if energy_added else 0
            location = session.get('chargingLocation', {}).get('formattedAddress', 'Unknown Location')
            latitude = session.get('chargingLocation', {}).get('mapMatchedLatitude', 0)
            longitude = session.get('chargingLocation', {}).get('mapMatchedLongitude', 0)
            avg_power = sum([block.get('averagePowerGridKw', 0) for block in session.get('chargingBlocks', [])]) / max(len(session.get('chargingBlocks', [])), 1)
            grid_power_start = [block.get('averagePowerGridKw', 0) for block in session.get('chargingBlocks', [])]
            mileage = session.get('mileage', 0)

            sessions.append({
                'start_time': start_time,
                'end_time': end_time,
                'soc_start': soc_start,
                'soc_end': soc_end,
                'energy_added': energy_added,
                'cost': cost,
                'efficiency': efficiency,
                'location': location,
                'latitude': latitude,
                'longitude': longitude,
                'avg_power': avg_power,
                'grid_power_start': grid_power_start,
                'mileage': mileage
            })
        except KeyError:
            continue
    return sessions

# Layout
app.layout = html.Div([
    html.H1('BMW CarData - Charging Session Dashboard', style={'textAlign': 'center', 'color': '#1f77b4'}),

    # Disclaimer
    html.Div([
        html.P(
            'Disclaimer: This application stores all uploaded data in memory, if you refresh your session is lost. Use at your own risk.',
            style={'textAlign': 'center', 'color': 'red', 'fontWeight': 'bold'}
        )
    ]),

    # File upload component
    html.Div([
        dcc.Upload(
            id='upload-json',
            children=html.Div([
                'Drag and Drop or ', html.A('Select a JSON File')
            ]),
            style={
                'width': '50%',
                'height': '60px',
                'lineHeight': '60px',
                'borderWidth': '1px',
                'borderStyle': 'dashed',
                'borderRadius': '5px',
                'textAlign': 'center',
                'margin': 'auto',
                'marginBottom': '20px',
            },
            multiple=False
        )
    ]),

    # Store component to hold session data
    dcc.Store(id='session-data'),

    # Total Energy Gauges Panel
    html.Div([
        dcc.Graph(
            id='total-energy-gauge',
            style={'height': '400px', 'width': '60%', 'display': 'inline-block', 'margin': '10px'}
        ),
        dcc.Graph(
            id='current-km-gauge',
            style={'height': '400px', 'width': '30%', 'display': 'inline-block', 'margin': '10px'}
        )
    ], style={'marginBottom': '20px', 'textAlign': 'center'}),

    # Overview Scatterplots
    html.Div([
        dcc.Graph(
            id='overview-scatterplot',
            style={'height': '400px', 'border': '2px solid #1f77b4', 'borderRadius': '10px', 'marginBottom': '20px'}
        ),
        dcc.Graph(
            id='average-gridpower-scatterplot',
            style={'height': '400px', 'border': '2px solid #1f77b4', 'borderRadius': '10px', 'marginBottom': '20px'}
        )
    ], style={'marginBottom': '20px'}),

    # Plaintext output
    html.Div([
        html.H3(id='session-info', style={'textAlign': 'center', 'color': '#1f77b4', 'marginBottom': '20px'})
    ]),

    # Dropdown to select session
    html.Div([
        html.Label('Select Charging Session by Time and Location:', style={'fontWeight': 'bold', 'color': '#1f77b4'}),
        dcc.Dropdown(
            id='session-dropdown',
            options=[],
            value=None,
            style={'border': '2px solid #1f77b4', 'borderRadius': '5px'}
        )
    ], style={'width': '50%', 'margin': 'auto', 'marginBottom': '20px'}),

    # Dashboard layout with compact and modern design
    html.Div([
        html.Div([
            html.Div([
                html.Iframe(id='range-map', style={'width': '100%', 'height': '300px', 'border': '2px solid #1f77b4', 'borderRadius': '10px', 'marginBottom': '10px'})
            ]),
            html.Div([
                html.Div([
                    dcc.Graph(
                        id='combined-gauges',
                        style={'height': '600px', 'border': '2px solid #1f77b4', 'borderRadius': '10px', 'marginBottom': '10px'}
                    )
                ])
            ])
        ], style={'flex': '1', 'paddingLeft': '10px'}),
        html.Div([
            dcc.Graph(id='charge-details-graph', style={'height': '300px', 'border': '2px solid #1f77b4', 'borderRadius': '10px', 'marginBottom': '10px'}),
            dcc.Graph(id='grid-power-graph', style={'height': '300px', 'border': '2px solid #1f77b4', 'borderRadius': '10px'})
        ], style={'flex': '1', 'paddingRight': '10px'})
    ], style={'display': 'flex', 'flexDirection': 'row', 'gap': '20px', 'alignItems': 'stretch'})
])

# Callbacks
@app.callback(
    [Output('session-dropdown', 'options'),
     Output('session-dropdown', 'value'),
     Output('total-energy-gauge', 'figure'),
     Output('current-km-gauge', 'figure'),
     Output('session-data', 'data')],
    [Input('upload-json', 'contents')]
)
def upload_json(contents):
    if contents:
        content_type, content_string = contents.split(',')
        decoded = base64.b64decode(content_string)
        data = json.loads(decoded)
        sessions = process_data(data)
        options = [
            {'label': f"{s['start_time'].strftime('%Y-%m-%d %H:%M')} - {s['location']}", 'value': i}
            for i, s in enumerate(sessions)
        ]
        total_energy_dc = sum(s['energy_added'] for s in sessions if s['avg_power'] >= 12)
        total_energy_ac = sum(s['energy_added'] for s in sessions if s['avg_power'] < 12)

        total_energy_fig = go.Figure()
        # Total DC Energy gauge
        total_energy_fig.add_trace(go.Indicator(
            mode="gauge+number",
            value=total_energy_dc,
            title={'text': "Total DC Energy (kWh)"},
            domain={'x': [0, 0.28], 'y': [0, 1]},  # Adjust domain to place it on the left with more space
            gauge={'axis': {'range': [0, total_energy_dc + total_energy_ac + 10]}, 'bar': {'color': "blue"}}
        ))

        # Total AC Energy gauge
        total_energy_fig.add_trace(go.Indicator(
            mode="gauge+number",
            value=total_energy_ac,
            title={'text': "Total AC Energy (kWh)"},
            domain={'x': [0.36, 0.64], 'y': [0, 1]},  # Adjust domain to place it in the middle with more space
            gauge={'axis': {'range': [0, total_energy_dc + total_energy_ac + 10]}, 'bar': {'color': "green"}}
        ))

        # Total Energy (AC + DC) gauge
        total_energy_fig.add_trace(go.Indicator(
            mode="gauge+number",
            value=total_energy_dc + total_energy_ac,
            title={'text': "Total Energy (AC + DC)"},
            domain={'x': [0.72, 1], 'y': [0, 1]},  # Adjust domain to place it on the right with more space
            gauge={'axis': {'range': [0, total_energy_dc + total_energy_ac + 20]}, 'bar': {'color': "purple"}}
        ))
        total_energy_fig.update_layout(height=400, width=900, template='plotly_white')

        current_km = max(s['mileage'] for s in sessions if s['mileage'] > 0)
        current_km_fig = go.Figure()
        current_km_fig.add_trace(go.Indicator(
            mode="gauge+number",
            value=current_km,
            title={'text': "Current km"},
            gauge={'axis': {'range': [0, current_km + 500]}, 'bar': {'color': "orange"}}
        ))
        current_km_fig.update_layout(height=400, width=300, template='plotly_white')

        return options, 0, total_energy_fig, current_km_fig, sessions
    return [], None, {}, {}, []

@app.callback(
    [Output('charge-details-graph', 'figure'),
     Output('session-info', 'children'),
     Output('combined-gauges', 'figure'),
     Output('grid-power-graph', 'figure'),
     Output('range-map', 'srcDoc'),
     Output('overview-scatterplot', 'figure'),
     Output('average-gridpower-scatterplot', 'figure')],
    [Input('session-dropdown', 'value'),
     State('session-data', 'data')]
)
def update_dashboard(selected_session, sessions):
    if selected_session is None or not sessions:
        return {}, "", {}, {}, "", {}, {}

    # Convert start_time and end_time back to datetime objects
    for session in sessions:
        session['start_time'] = datetime.datetime.fromisoformat(session['start_time'])
        session['end_time'] = datetime.datetime.fromisoformat(session['end_time'])

    session = sessions[selected_session]

    # Charge details graph
    charge_details_fig = go.Figure()
    charge_details_fig.add_trace(go.Scatter(
        x=[session['start_time'], session['end_time']],
        y=[session['soc_start'], session['soc_end']],
        mode='lines+markers',
        name='SOC',
        marker=dict(color='blue')
    ))
    charge_details_fig.update_layout(
        title='Charge Details',
        xaxis_title='Time',
        yaxis_title='SOC (%)',
        xaxis=dict(showgrid=True, zeroline=True),
        yaxis=dict(showgrid=True, zeroline=True),
        template='plotly_white'
    )

    # Session info text
    session_info = f"Energy Added: {session['energy_added']} kWh, Cost: €{session['cost']}, Efficiency: {session['efficiency']:.2%}, Location: {session['location']}"

    # Combined gauges
    combined_gauges = go.Figure()

    # Average Grid Power gauge
    combined_gauges.add_trace(go.Indicator(
        mode="gauge+number",
        value=session['avg_power'],
        title={'text': "Average Grid Power (kW)"},
        domain={'x': [0, 0.45], 'y': [0.6, 1]},  # Adjust domain to leave space on the right
        gauge={
            'axis': {'range': [None, max([s['avg_power'] for s in sessions]) + 10]},
            'bar': {'color': "darkblue"},
            'steps': [
                {'range': [0, session['avg_power'] / 2], 'color': "lightblue"},
                {'range': [session['avg_power'] / 2, session['avg_power']], 'color': "blue"}
            ]
        }
    ))

    # Cost gauge
    combined_gauges.add_trace(go.Indicator(
        mode="gauge+number",
        value=session['cost'],
        title={'text': "Cost (€)"},
        domain={'x': [0.55, 1], 'y': [0.6, 1]},  # Adjust domain to leave space on the left
        gauge={
            'axis': {'range': [0, max([s['cost'] for s in sessions]) + 10]},
            'bar': {'color': "green"},
        }
    ))

    # Efficiency gauge
    combined_gauges.add_trace(go.Indicator(
        mode="gauge+number",
        value=session['efficiency'] * 100,
        title={'text': "Efficiency (%)"},
        domain={'x': [0, 0.45], 'y': [0.2, 0.6]},  # Adjust domain to leave space on the right
        gauge={
            'axis': {'range': [0, 100]},
            'bar': {'color': "orange"},
        }
    ))

    # Energy Added gauge
    combined_gauges.add_trace(go.Indicator(
        mode="gauge+number",
        value=session['energy_added'],
        title={'text': "Energy Added (kWh)"},
        domain={'x': [0.55, 1], 'y': [0.2, 0.6]},  # Adjust domain to leave space on the left
        gauge={
            'axis': {'range': [0, max([s['energy_added'] for s in sessions]) + 10]},
            'bar': {'color': "purple"},
        }
    ))

    combined_gauges.update_layout(
        template='plotly_white',
        height=600
    )

    # Grid Power over Time graph
    grid_power_fig = go.Figure()
    grid_power_fig.add_trace(go.Scatter(
        x=[session['start_time'] + datetime.timedelta(seconds=i * (session['end_time'] - session['start_time']).total_seconds() / len(session['grid_power_start'])) for i in range(len(session['grid_power_start']))],
        y=session['grid_power_start'],
        mode='lines+markers',
        name='Grid Power',
        marker=dict(color='green')
    ))
    grid_power_fig.update_layout(
        title='Grid Power Over Time',
        xaxis_title='Time',
        yaxis_title='Grid Power (kW)',
        xaxis=dict(showgrid=True, zeroline=True),
        yaxis=dict(showgrid=True, zeroline=True),
        template='plotly_white'
    )

    # Overview scatterplot
    overview_fig = go.Figure()
    for s in sessions:
        overview_fig.add_trace(go.Scatter(
            x=[s['start_time']],
            y=[s['energy_added']],
            mode='markers',
            marker=dict(size=10, color='blue'),
            name=f"{s['start_time'].strftime('%Y-%m-%d %H:%M')} - {s['location']}"
        ))
    overview_fig.update_layout(
        title='Overview of All Charging Sessions',
        xaxis_title='Start Time',
        yaxis_title='Energy Added (kWh)',
        template='plotly_white'
    )

    # Average Grid Power scatterplot
    avg_gridpower_fig = go.Figure()
    for session in sessions:
        avg_gridpower_fig.add_trace(go.Scatter(
            x=[i for i in range(len(session['grid_power_start']))],
            y=session['grid_power_start'],
            mode='markers',
            marker=dict(size=6, color=session['grid_power_start'], colorscale='Viridis', showscale=False),
            name=f"Session {session['start_time']}"
        ))
    avg_gridpower_fig.update_layout(
        title='Average Grid Power Across All Sessions',
        xaxis_title='Session Progress',
        yaxis_title='Grid Power (kW)',
        template='plotly_white',
        showlegend=False
    )

    # Generate Folium map
    selected_session = sessions[selected_session]  # Default to the first session if none is selected
    zoom_level = 13 if selected_session['grid_power_start'] else 5  # Zoom in if a session is selected, otherwise use default zoom level
    m = Map(location=[selected_session['latitude'], selected_session['longitude']], zoom_start=zoom_level, tiles="https://tiles.ext.ffmuc.net/osm/{z}/{x}/{y}.png", attr="OpenStreetMap")
    for session in sessions:
        Marker([session['latitude'], session['longitude']], popup=session['location']).add_to(m)
    map_html = m._repr_html_()
    # Render the map to an HTML string
    map_html = io.BytesIO()
    m.save(map_html, close_file=False)
    map_html.seek(0)
    map_html_content = map_html.read().decode('utf-8')

    return charge_details_fig, session_info, combined_gauges, grid_power_fig, map_html_content, overview_fig, avg_gridpower_fig

# Run the app
if __name__ == '__main__':
    app.run_server(debug=True, port=8050, threaded=True, host='0.0.0.0')