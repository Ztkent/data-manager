from pyvis.network import Network

import db
import json

def save_pyvis_link_graph(urls: db.LinkData, enable_physics: bool):
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
        if referrer == "STARTING_URL":
            continue
        G.add_node(referrer)
        G.add_node(url)
        G.add_edge(referrer, url)

    # Display the graph
    G.save_graph("html/network.html")