from pyvis.network import Network

import db
import json

import networkx as nx
import plotly.graph_objects as go


def save_networkx_link_graph(urls: db.LinkData, output_file: str) -> None:
    urls = [(link.referrer, link.url) for link in urls[1:]]
    G = nx.DiGraph()

    if G is None:
        raise ValueError("Failed to create Network object")

    for referrer, url in urls:
        if referrer == "STARTING_URL":
            continue
        G.add_edge(referrer, url)

    pos = nx.spring_layout(G)

    edge_x = []
    edge_y = []
    for edge in G.edges():
        x0, y0 = pos[edge[0]]
        x1, y1 = pos[edge[1]]
        edge_x.extend([x0, x1, None])
        edge_y.extend([y0, y1, None])

    edge_trace = go.Scatter(
        x=edge_x, y=edge_y,
        line=dict(width=0.5, color='#888'),
        hoverinfo='none',
        mode='lines')

    node_x = []
    node_y = []
    for node in G.nodes():
        x, y = pos[node]
        node_x.append(x)
        node_y.append(y)

    node_trace = go.Scatter(
        x=node_x, y=node_y,
        mode='markers',
        hoverinfo='text',
        marker=dict(
            showscale=True,
            colorscale='YlGnBu',
            reversescale=True,
            color=[],
            size=10,
            colorbar=dict(
                thickness=15,
                title='Node Connections',
                xanchor='left',
                titleside='right'
            ),
            line=dict(width=2)))

    fig = go.Figure(data=[edge_trace, node_trace],
                    layout=go.Layout(
                        title='<br>Network graph',
                        titlefont=dict(size=16),
                        showlegend=False,
                        hovermode='closest',
                        margin=dict(b=20, l=5, r=5, t=40),
                        annotations=[dict(
                            text="Python code: <a href='https://plotly.com/ipython-notebooks/network-graphs/'> https://plotly.com/ipython-notebooks/network-graphs/</a>",
                            showarrow=False,
                            xref="paper", yref="paper",
                            x=0.005, y=-0.002)],
                        xaxis=dict(showgrid=False, zeroline=False, showticklabels=False),
                        yaxis=dict(showgrid=False, zeroline=False, showticklabels=False)))

    fig.write_html(output_file)


def save_pyvis_link_graph(urls: db.LinkData, enable_physics, output_file: str) -> None:
    urls = [(link.referrer, link.url) for link in urls[1:]]
    G = Network()
    if G is None:
        raise ValueError("Failed to create Network object")
    
    # Determine which options to use
    # https://pyvis.readthedocs.io/en/latest/documentation.html#pyvis.network.Network.set_options

    if not enable_physics:
        options = {
            "physics": {
                "enabled": False
            },
            "interaction": {
                "navigationButtons": True,
                "keyboard": True,
                "zoomView": True,
                "dragNodes": True
            },
            "edges": {
                "color": "blue",
                "width": 2
            }
        }
    else:
        options = {
            "physics": {
                "barnesHut": {
                    "gravitationalConstant": -2000,
                    "centralGravity": 0.3,
                    "springLength": 95,
                    "springConstant": 0.04,
                    "damping": 0.09,
                    "avoidOverlap": 0
                },
                "minVelocity": 0.75
            },
            "interaction": {
                "navigationButtons": True,
                "keyboard": True,
                "zoomView": True,
                "dragNodes": True
            },
            "edges": {
                "color": "blue",
                "width": 2
            }
        }
    
    G.set_options(json.dumps(options))    
    for referrer, url in urls:
        G.add_node(referrer)
        G.add_node(url)
        G.add_edge(referrer, url)

    # Display the graph
    G.save_graph(output_file)