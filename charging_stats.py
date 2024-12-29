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
app = dash.Dash()
app.title = 'BMW CarData - Charging Session Dashboard'
app.css.config.serve_locally = True
app.scripts.config.serve_locally = True
server = app.server

# Set maximum file upload size (e.g., 5MB)
app.server.config['MAX_CONTENT_LENGTH'] = 5 * 1024 * 1024

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

# Function to create a gauge trace
def create_gauge_trace(value, title, color, domain_x, domain_y=[0, 1], range_max=None):
    return go.Indicator(
        mode="gauge+number",
        value=value,
        title={'text': title},
        domain={'x': domain_x, 'y': domain_y},
        gauge={'axis': {'range': [0, range_max] if range_max else [None, None]}, 'bar': {'color': color}}
    )

# Function to create a scatter plot
def create_scatter_plot(x, y, title, xaxis_title, yaxis_title, color='blue', mode='markers', size=10):
    fig = go.Figure()
    fig.add_trace(go.Scatter(
        x=x,
        y=y,
        mode=mode,
        marker=dict(size=size, color=color),
        name=title
    ))
    fig.update_layout(
        title=title,
        xaxis_title=xaxis_title,
        yaxis_title=yaxis_title,
        template='plotly_white'
    )
    return fig

# Function to create a Folium map
def create_folium_map(sessions, selected_session=None):
    if selected_session:
        zoom_level = 13
        center = [selected_session['latitude'], selected_session['longitude']]
    else:
        zoom_level = 5
        center = [sessions[0]['latitude'], sessions[0]['longitude']] if sessions else [0, 0]

    m = Map(location=center, zoom_start=zoom_level, tiles="https://tiles.ext.ffmuc.net/osm/{z}/{x}/{y}.png", attr="OpenStreetMap")
    for session in sessions:
        Marker([session['latitude'], session['longitude']], popup=session['location']).add_to(m)
    map_html = m._repr_html_()
    return map_html

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

    # Button to load demo data
    html.Div([
        html.Button('Load Demo Data', id='load-demo-data', n_clicks=0, style={'marginBottom': '20px'})
    ], style={'textAlign': 'center'}),

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
    [Input('upload-json', 'contents'),
     Input('load-demo-data', 'n_clicks')]
)
def upload_json(contents, n_clicks):
    if contents:
        content_type, content_string = contents.split(',')
        decoded = base64.b64decode(content_string)
        try:
            data = json.loads(decoded)
        except json.JSONDecodeError:
            return [], None, {}, {}, []
    elif n_clicks > 0:
        with open('FINAL_DEMO_CHARGING_DATA_SMOOTH_CURVES.JSON', 'r') as f:
            data = json.load(f)
    else:
        return [], None, {}, {}, []

    sessions = process_data(data)
    options = [
        {'label': f"{s['start_time'].strftime('%Y-%m-%d %H:%M')} - {s['location']}", 'value': i}
        for i, s in enumerate(sessions)
    ]
    total_energy_dc = sum(s['energy_added'] for s in sessions if s['avg_power'] >= 12)
    total_energy_ac = sum(s['energy_added'] for s in sessions if s['avg_power'] < 12)

    total_energy_fig = go.Figure()
    total_energy_fig.add_trace(create_gauge_trace(total_energy_dc, "Total DC Energy (kWh)", "blue", [0, 0.28], range_max=total_energy_dc + total_energy_ac + 10))
    total_energy_fig.add_trace(create_gauge_trace(total_energy_ac, "Total AC Energy (kWh)", "green", [0.36, 0.64], range_max=total_energy_dc + total_energy_ac + 10))
    total_energy_fig.add_trace(create_gauge_trace(total_energy_dc + total_energy_ac, "Total Energy (AC + DC)", "purple", [0.72, 1], range_max=total_energy_dc + total_energy_ac + 20))
    total_energy_fig.update_layout(height=400, width=900, template='plotly_white')

    current_km = max(s['mileage'] for s in sessions if s['mileage'] > 0)
    current_km_fig = go.Figure()
    current_km_fig.add_trace(create_gauge_trace(current_km, "Current km", "orange", [0, 1], range_max=current_km + 500))
    current_km_fig.update_layout(height=400, width=300, template='plotly_white')

    return options, 0, total_energy_fig, current_km_fig, sessions

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
    session_info = f"Energy Added: {session['energy_added']} kWh, Cost: €{session['cost']}, Efficiency: {session['efficiency']:.2%}, Location: {session['location']}"

    # Combined gauges
    combined_gauges = go.Figure()
    combined_gauges.add_trace(create_gauge_trace(session['avg_power'], "Average Grid Power (kW)", "darkblue", [0, 0.45], [0.6, 1], max([s['avg_power'] for s in sessions]) + 10))
    combined_gauges.add_trace(create_gauge_trace(session['cost'], "Cost (€)", "green", [0.55, 1], [0.6, 1], max([s['cost'] for s in sessions]) + 10))
    combined_gauges.add_trace(create_gauge_trace(session['efficiency'] * 100, "Efficiency (%)", "orange", [0, 0.45], [0.2, 0.6], 100))
    combined_gauges.add_trace(create_gauge_trace(session['energy_added'], "Energy Added (kWh)", "purple", [0.55, 1], [0.2, 0.6], max([s['energy_added'] for s in sessions]) + 10))
    combined_gauges.update_layout(template='plotly_white', height=600)

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
            mode='markers',
            marker=dict(size=6, color=s['grid_power_start'], colorscale='Viridis', showscale=False),
            name=f"Session {s['start_time']}"
        ))
    avg_gridpower_fig.update_layout(
        title='Average Grid Power Across All Sessions',
        xaxis_title='Session Time',
        yaxis_title='Grid Power (kW)',
        template='plotly_white',
        showlegend=False
    )

    # Generate Folium map
    map_html_content = create_folium_map(sessions, session)

    return charge_details_fig, session_info, combined_gauges, grid_power_fig, map_html_content, overview_fig, avg_gridpower_fig

# Run the app
if __name__ == '__main__':
    app.run_server(debug=False, port=8050, threaded=True, host='0.0.0.0')