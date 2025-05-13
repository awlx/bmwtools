import dash
from dash import dcc, html
from flask import session
import uuid
from callbacks import register_callbacks
from utils import get_disclaimer_with_hash

# Initialize Dash app
app = dash.Dash()
app.title = 'BMW CarData - Charging Session Dashboard'
app.css.config.serve_locally = True
app.scripts.config.serve_locally = True

# Set maximum file upload size (e.g., 5MB)
app.server.config['MAX_CONTENT_LENGTH'] = 5 * 1024 * 1024
app.server.secret_key = str(uuid.uuid4())  # Set a secret key for session management

# Generate a unique session ID for each user session
@app.server.before_request
def make_session_permanent():
    session.permanent = True
    if 'session_id' not in session:
        session['session_id'] = str(uuid.uuid4())

# Layout
app.layout = html.Div([
    html.H1('BMW CarData - Charging Session Dashboard', style={'textAlign': 'center', 'color': '#1f77b4'}),

    # Disclaimer
    html.Div([
        dcc.Markdown(
            get_disclaimer_with_hash(),
            style={'textAlign': 'center', 'color': 'red', 'fontWeight': 'bold', "white-space": "pre"}
        )
    ]),
    
    # Warning message for estimated values
    html.Div(id='energy-data-warning', style={'textAlign': 'center', 'color': 'orange', 'fontWeight': 'bold', 'margin': '10px', 'display': 'none'}),

    # File upload component
    html.Div([
        dcc.Upload(
            id='upload-json',
            children=html.Div([
                'Drag and Drop or ', html.A('Select your CarData JSON file (BMW-CarData-Ladehistorie_*.json)')
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
                'marginBottom': '10px',  # Reduced margin
            },
            multiple=False
        )
    ]),

    # Button to load demo data
    html.Div([
        html.Button('Load Demo Data', id='load-demo-data', n_clicks=0, style={'marginBottom': '10px'})  # Reduced margin
    ], style={'textAlign': 'center'}),

    # Store component to hold session data
    dcc.Store(id='session-data'),

    # Datepicker to select time range
    html.Div([
        html.Label('Select Date Range for analysis (optional):', style={'fontWeight': 'bold', 'color': '#1f77b4'}),
        dcc.DatePickerRange(
            id='date-picker-range',
            start_date_placeholder_text='Start Date',
            end_date_placeholder_text='End Date',
            display_format='YYYY-MM-DD',
            style={'border': '2px solid #1f77b4', 'borderRadius': '5px', 'marginBottom': '10px'}  # Reduced margin
        )
    ], style={'width': '50%', 'margin': 'auto', 'marginBottom': '10px'}),  # Reduced margin
  
    # Miles / kilometers toggle button
    html.Div([
        html.Button('Toggle Units (km/miles)', id='toggle-units', n_clicks=0, style={'marginBottom': '10px'})  # New button
    ], style={'textAlign': 'center'}),
    
    # Total Energy Gauges Panel
    html.Div([
        dcc.Graph(
            id='total-energy-gauge',
            style={'height': '300px', 'width': '60%', 'display': 'inline-block', 'margin': '10px'}
        ),
        dcc.Graph(
            id='current-km-gauge',
            style={'height': '300px', 'width': '30%', 'display': 'inline-block', 'margin': '10px'}
        )
    ], style={'marginBottom': '10px', 'textAlign': 'center', 'clear': 'both'}),  # Reduced margin

    # Overall Efficiency and Power Consumption Gauges Panel
    html.Div([
        dcc.Graph(id='overall-efficiency-gauge', style={'height': '300px', 'width': '30%', 'display': 'inline-block'}),
        dcc.Graph(id='power-consumption-gauge', style={'height': '300px', 'width': '30%', 'display': 'inline-block'}),
        dcc.Graph(id='power-consumption-without-grid-losses-gauge', style={'height': '300px', 'width': '30%', 'display': 'inline-block'})
    ], style={'marginBottom': '10px', 'textAlign': 'center', 'clear': 'both'}),  # Reduced margin
    
    # Session Stats Gauges Panel
    html.Div([
        dcc.Graph(id='total-sessions-gauge', style={'height': '300px', 'width': '30%', 'display': 'inline-block'}),
        dcc.Graph(id='successful-sessions-gauge', style={'height': '300px', 'width': '30%', 'display': 'inline-block'}),
        dcc.Graph(id='failed-sessions-gauge', style={'height': '300px', 'width': '30%', 'display': 'inline-block'}),
    ], style={'marginBottom': '10px', 'textAlign': 'center', 'clear': 'both'}),  # Reduced margin

    # Top Providers
    html.Div([
        html.Div([
            html.H4("Top 5 Successful Providers"),
            html.Ul(id='top-successful-providers', style={'listStyleType': 'none', 'padding': '0'})
        ], style={'width': '45%', 'display': 'inline-block', 'verticalAlign': 'top'}),
        html.Div([
            html.H4("Top 5 Failed Providers"),
            html.Ul(id='top-failed-providers', style={'listStyleType': 'none', 'padding': '0'})
        ], style={'width': '45%', 'display': 'inline-block', 'verticalAlign': 'top', 'marginRight': '5%'}),
    ], style={'textAlign': 'center', 'marginBottom': '10px', 'clear': 'both'}),  # Reduced margin
    
    # SOC Statistics
    html.Div([
        html.H4("SOC Statistics"),
        html.Div(id='soc-stats', style={'textAlign': 'center', 'marginBottom': '10px', 'listStyleType': 'none'})  # New section for SOC statistics
    ], style={'textAlign': 'center', 'marginBottom': '10px', 'clear': 'both'}),  # Reduced margin

    # Overview Scatterplots
    html.Div([
        dcc.Graph(
            id='overview-scatterplot',
            style={'height': '400px', 'border': '2px solid #1f77b4', 'borderRadius': '10px', 'marginBottom': '10px'}  # Reduced margin
        ),
        dcc.Graph(
            id='average-gridpower-scatterplot',
            style={'height': '400px', 'border': '2px solid #1f77b4', 'borderRadius': '10px', 'marginBottom': '10px'}  # Reduced margin
        ),
        dcc.Graph(
            id='estimated-battery-capacity-scatterplot',
            style={'height': '400px', 'border': '2px solid #1f77b4', 'borderRadius': '10px', 'marginBottom': '10px'}  # Reduced margin
        )
    ], style={'marginBottom': '10px'}),  # Reduced margin

    # Second map for charging locations
    html.Div([
        html.Iframe(id='charging-locations-map', style={'width': '100%', 'height': '400px', 'border': '2px solid #1f77b4', 'borderRadius': '10px', 'marginBottom': '10px'})  # Reduced margin
    ], style={'marginBottom': '10px'}),  # Reduced margin

    # Dropdown to select session
    html.Div([
        html.Label('Select Charging Session by Time and Location:', style={'fontWeight': 'bold', 'color': '#1f77b4'}),
        dcc.Dropdown(
            id='session-dropdown',
            options=[],
            value=None,
            style={'border': '2px solid #1f77b4', 'borderRadius': '5px'}
        )
    ], style={'width': '50%', 'margin': 'auto', 'marginBottom': '10px'}),  # Reduced margin

    # Plaintext output
    html.Div([
        html.H3(id='session-info', style={'textAlign': 'center', 'color': '#1f77b4', 'marginBottom': '10px'})  # Reduced margin
    ]),


    # Dashboard layout with compact and modern design
    html.Div([
        html.Div([
            html.Div([
                html.Iframe(id='range-map', style={'width': '100%', 'height': '300px', 'border': '2px solid #1f77b4', 'borderRadius': '10px', 'marginBottom': '10px'})  # Reduced margin
            ]),
            html.Div([
                html.Div([
                    dcc.Graph(
                        id='combined-gauges',
                        style={'height': '800px', 'border': '2px solid #1f77b4', 'borderRadius': '10px', 'marginBottom': '10px'})  # Reduced margin
                ])
            ])
        ], style={'flex': '1', 'paddingLeft': '10px'}),
        html.Div([
            dcc.Graph(id='charge-details-graph', style={'height': '300px', 'border': '2px solid #1f77b4', 'borderRadius': '10px', 'marginBottom': '10px'}),  # Reduced margin
            dcc.Graph(id='grid-power-graph', style={'height': '300px', 'border': '2px solid #1f77b4', 'borderRadius': '10px'})  # Reduced margin
        ], style={'flex': '1', 'paddingRight': '10px'})
    ], style={'display': 'flex', 'flexDirection': 'row', 'gap': '20px', 'alignItems': 'stretch'}),
    
    # Imprint
    html.Div([
        html.P(
            'Imprint: Freie Netze München e. V. / Parkstraße 28 / 82131 Gauting',
            style={'textAlign': 'center', 'color': 'black', 'fontWeight': 'bold'}
        )
    ]),
])

# Register callbacks
register_callbacks(app)

# Run the app
if __name__ == '__main__':
    app.run_server(debug=False, port=8050, threaded=True, host='0.0.0.0')
